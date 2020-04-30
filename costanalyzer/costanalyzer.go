// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"git.arvados.org/arvados.git/sdk/go/arvados"
	"git.arvados.org/arvados.git/sdk/go/arvadosclient"
	"git.arvados.org/arvados.git/sdk/go/keepclient"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Dict is a helper type so we don't have to write out 'map[string]interface{}' every time.
type Dict map[string]interface{}

// LegacyNodeInfo is a struct for records created by Arvados Node Manager (Arvados <= 1.4.3)
// Example:
// {
//    "total_cpu_cores":2,
//    "total_scratch_mb":33770,
//    "cloud_node":
//      {
//        "price":0.1,
//        "size":"m4.large"
//      },
//     "total_ram_mb":7986
// }
type LegacyNodeInfo struct {
	CPUCores  int64           `json:"total_cpu_cores"`
	ScratchMb int64           `json:"total_scratch_mb"`
	RAMMb     int64           `json:"total_ram_mb"`
	CloudNode LegacyCloudNode `json:"cloud_node"`
}

// LegacyCloudNode is a struct for records created by Arvados Node Manager (Arvados <= 1.4.3)
type LegacyCloudNode struct {
	Price float64 `json:"price"`
	Size  string  `json:"size"`
}

// Node is a struct for records created by Arvados Dispatch Cloud (Arvados >= 2.0.0)
// Example:
// {
//    "Name": "Standard_D1_v2",
//    "ProviderType": "Standard_D1_v2",
//    "VCPUs": 1,
//    "RAM": 3584000000,
//    "Scratch": 50000000000,
//    "IncludedScratch": 50000000000,
//    "AddedScratch": 0,
//    "Price": 0.057,
//    "Preemptible": false
//}
type Node struct {
	VCPUs        int64
	Scratch      int64
	RAM          int64
	Price        float64
	Name         string
	ProviderType string
	Preemptible  bool
}

type report struct {
	Type string
	Msg  string
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return ""
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func logError(m []string) {
	log.Print(string(marshal(report{"Error", strings.Join(m, " ")})))
}

func marshal(message interface{}) (encoded []byte) {
	encoded, err := json.Marshal(message)
	if err != nil {
		// do not call logError here because that would create an infinite loop
		fmt.Fprintln(os.Stderr, "{\"Error\": \"Unable to marshal message into json:", message, "\"}")
		return nil
	}
	return
}

func parseFlags() (uuids arrayFlags) {

	flags := flag.NewFlagSet("cost-analyzer", flag.ExitOnError)
	flags.Var(&uuids, "uuid", "Toplevel project or container request uuid. May be specified more than once.")

	flags.Usage = func() { usage(flags) }

	// Parse args; omit the first arg which is the command name
	err := flags.Parse(os.Args[1:])
	if err != nil {
		logError([]string{"Unable to parse command line arguments:", err.Error()})
		os.Exit(1)
	}

	if len(uuids) == 0 {
		usage(flags)
		os.Exit(1)
	}

	return
}

func ensureDirectory(dir string) {
	statData, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			logError([]string{"Error creating directory", dir, ":", err.Error()})
			os.Exit(1)
		}
	} else {
		if !statData.IsDir() {
			logError([]string{"The path", dir, "is not a directory"})
			os.Exit(1)
		}
	}
}

func addContainerLine(node interface{}, cr Dict, container Dict) (csv string, cost float64) {
	csv = cr["uuid"].(string) + ","
	csv += cr["name"].(string) + ","
	csv += container["uuid"].(string) + ","
	csv += container["state"].(string) + ","
	if container["started_at"] != nil {
		csv += container["started_at"].(string) + ","
	} else {
		csv += ","
	}

	var delta time.Duration
	if container["finished_at"] != nil {
		csv += container["finished_at"].(string) + ","
		finishedTimestamp, err := time.Parse("2006-01-02T15:04:05.000000000Z", container["finished_at"].(string))
		if err != nil {
			fmt.Println(err)
		}
		startedTimestamp, err := time.Parse("2006-01-02T15:04:05.000000000Z", container["started_at"].(string))
		if err != nil {
			fmt.Println(err)
		}
		delta = finishedTimestamp.Sub(startedTimestamp)
		csv += strconv.FormatFloat(delta.Seconds(), 'f', 0, 64) + ","
	} else {
		csv += ",,"
	}
	var price float64
	var size string
	switch n := node.(type) {
	case Node:
		price = n.Price
		size = n.ProviderType
	case LegacyNodeInfo:
		price = n.CloudNode.Price
		size = n.CloudNode.Size
	default:
		log.Printf("WARNING: unknown node type found!")
	}
	cost = delta.Seconds() / 3600 * price
	csv += size + "," + strconv.FormatFloat(price, 'f', 8, 64) + "," + strconv.FormatFloat(cost, 'f', 8, 64) + "\n"
	return
}

func loadCachedObject(file string, uuid string) (reload bool, object Dict) {
	reload = true
	// See if we have a cached copy of this object
	if _, err := os.Stat(file); err == nil {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			log.Printf("error reading %q: %s", file, err)
			return
		}
		err = json.Unmarshal(data, &object)
		if err != nil {
			log.Printf("failed to unmarshal json: %s: %s", data, err)
			return
		}

		// See if it is in a final state, if that makes sense
		// Projects (j7d0g) do not have state so they should always be reloaded
		if !strings.Contains(uuid, "-j7d0g-") {
			if object["state"].(string) == "Complete" || object["state"].(string) == "Failed" {
				reload = false
				return
			}
		}
	}
	return
}

// Load an Arvados object.
func loadObject(arv *arvadosclient.ArvadosClient, path string, uuid string) (object Dict) {

	ensureDirectory(path)

	file := path + "/" + uuid + ".json"

	var reload bool
	reload, object = loadCachedObject(file, uuid)

	if reload {
		var err error
		if strings.Contains(uuid, "-d1hrv-") {
			err = arv.Get("pipeline_instances", uuid, nil, &object)
		} else if strings.Contains(uuid, "-j7d0g-") {
			err = arv.Get("groups", uuid, nil, &object)
		} else if strings.Contains(uuid, "-xvhdp-") {
			err = arv.Get("container_requests", uuid, nil, &object)
		} else if strings.Contains(uuid, "-dz642-") {
			err = arv.Get("containers", uuid, nil, &object)
		} else {
			err = arv.Get("jobs", uuid, nil, &object)
		}
		if err != nil {
			logError([]string{fmt.Sprintf("error loading object with UUID %q: %s", uuid, err)})
			os.Exit(1)
		}
		encoded, err := json.MarshalIndent(object, "", " ")
		if err != nil {
			logError([]string{fmt.Sprintf("error marshaling object with UUID %q: %s", uuid, err)})
			os.Exit(1)
		}
		err = ioutil.WriteFile(file, encoded, 0644)
		if err != nil {
			logError([]string{fmt.Sprintf("error writing file %s: %s", file, err)})
			os.Exit(1)
		}
	}
	return
}

func getNode(arv *arvadosclient.ArvadosClient, arv2 *arvados.Client, kc *keepclient.KeepClient, itemMap Dict) (node interface{}, err error) {
	if _, ok := itemMap["log_uuid"]; ok {
		if itemMap["log_uuid"] == nil {
			err = errors.New("No log collection")
			return
		}

		var collection arvados.Collection
		err = arv.Get("collections", itemMap["log_uuid"].(string), nil, &collection)
		if err != nil {
			log.Printf("error getting collection: %s\n", err)
			return
		}

		var fs arvados.CollectionFileSystem
		fs, err = collection.FileSystem(arv2, kc)
		if err != nil {
			log.Printf("error opening collection as filesystem: %s\n", err)
			return
		}
		var f http.File
		f, err = fs.Open("node.json")
		if err != nil {
			log.Printf("error opening file in collection: %s\n", err)
			return
		}

		var nodeDict Dict
		// TODO: checkout io (ioutil?) readall function
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(f)
		if err != nil {
			log.Printf("error reading %q: %s\n", f, err)
			return
		}
		contents := buf.String()
		f.Close()

		err = json.Unmarshal([]byte(contents), &nodeDict)
		if err != nil {
			log.Printf("error unmarshalling: %s\n", err)
			return
		}
		if val, ok := nodeDict["properties"]; ok {
			var encoded []byte
			encoded, err = json.MarshalIndent(val, "", " ")
			if err != nil {
				log.Printf("error marshalling: %s\n", err)
				return
			}
			// node is type LegacyNodeInfo
			var newNode LegacyNodeInfo
			err = json.Unmarshal(encoded, &newNode)
			if err != nil {
				log.Printf("error unmarshalling: %s\n", err)
				return
			}
			node = newNode
		} else {
			// node is type Node
			var newNode Node
			err = json.Unmarshal([]byte(contents), &newNode)
			if err != nil {
				log.Printf("error unmarshalling: %s\n", err)
				return
			}
			node = newNode
		}
	}
	return
}

func handleProject(uuid string, arv *arvadosclient.ArvadosClient, arv2 *arvados.Client, kc *keepclient.KeepClient) (cost map[string]float64) {

	cost = make(map[string]float64)

	project := loadObject(arv, "results"+"/"+uuid, uuid)

	// arv -f uuid container_request list --filters '[["owner_uuid","=","<someuuid>"],["requesting_container_uuid","=",null]]'

	// Now find all container requests that have the container we found above as requesting_container_uuid
	var childCrs map[string]interface{}
	filterset := []arvados.Filter{
		{
			Attr:     "owner_uuid",
			Operator: "=",
			Operand:  project["uuid"].(string),
		},
		{
			Attr:     "requesting_container_uuid",
			Operator: "=",
			Operand:  nil,
		},
	}
	err := arv.List("container_requests", arvadosclient.Dict{"filters": filterset, "limit": 10000}, &childCrs)
	if err != nil {
		log.Fatal("error querying container_requests", err.Error())
	}
	if value, ok := childCrs["items"]; ok {
		log.Println("Collecting top level container requests in project")
		items := value.([]interface{})
		for _, item := range items {
			itemMap := item.(map[string]interface{})
			for k, v := range generateCrCsv(itemMap["uuid"].(string), arv, arv2, kc) {
				cost[k] = v
			}
		}
	}
	return
}

func generateCrCsv(uuid string, arv *arvadosclient.ArvadosClient, arv2 *arvados.Client, kc *keepclient.KeepClient) (cost map[string]float64) {

	cost = make(map[string]float64)

	csv := "CR UUID,CR name,Container UUID,State,Started At,Finished At,Duration in seconds,Compute node type,Hourly node cost,Total cost\n"
	var tmpCsv string
	var tmpTotalCost float64
	var totalCost float64

	// This is a container request, find the container
	cr := loadObject(arv, "results"+"/"+uuid, uuid)
	container := loadObject(arv, "results"+"/"+uuid, cr["container_uuid"].(string))

	topNode, err := getNode(arv, arv2, kc, cr)
	if err != nil {
		log.Fatalf("error getting node: %s", err)
	}
	tmpCsv, totalCost = addContainerLine(topNode, cr, container)
	csv += tmpCsv
	totalCost += tmpTotalCost

	cost[container["uuid"].(string)] = totalCost

	// Now find all container requests that have the container we found above as requesting_container_uuid
	var childCrs map[string]interface{}
	filterset := []arvados.Filter{
		{
			Attr:     "requesting_container_uuid",
			Operator: "=",
			Operand:  container["uuid"].(string),
		}}
	err = arv.List("container_requests", arvadosclient.Dict{"filters": filterset, "limit": 10000}, &childCrs)
	if err != nil {
		log.Fatal("error querying container_requests", err.Error())
	}
	if value, ok := childCrs["items"]; ok {
		log.Println("Collecting child containers")
		items := value.([]interface{})
		for _, item := range items {
			fmt.Fprintf(os.Stderr, ".")
			itemMap := item.(map[string]interface{})
			node, _ := getNode(arv, arv2, kc, itemMap)
			c2 := loadObject(arv, "results"+"/"+uuid, itemMap["container_uuid"].(string))
			tmpCsv, tmpTotalCost = addContainerLine(node, itemMap, c2)
			cost[itemMap["container_uuid"].(string)] = tmpTotalCost
			csv += tmpCsv
			totalCost += tmpTotalCost
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	log.Println("Done")

	csv += "TOTAL,,,,,,,,," + strconv.FormatFloat(totalCost, 'f', 8, 64) + "\n"

	// Write the resulting CSV file
	err = ioutil.WriteFile("results"+"/"+uuid+".csv", []byte(csv), 0644)
	if err != nil {
		logError([]string{"Error writing file", ":", err.Error()})
		os.Exit(1)
	}

	log.Println("Results in results/" + uuid + ".csv")
	return
}

func main() {

	uuids := parseFlags()

	ensureDirectory("results")

	// Arvados Client setup
	arv, err := arvadosclient.MakeArvadosClient()
	if err != nil {
		logError([]string{fmt.Sprintf("error creating Arvados object: %s", err)})
		os.Exit(1)
	}
	kc, err := keepclient.MakeKeepClient(arv)
	if err != nil {
		logError([]string{fmt.Sprintf("error creating Keep object: %s", err)})
		os.Exit(1)
	}

	arv2 := arvados.NewClientFromEnv()

	cost := make(map[string]float64)

	for _, uuid := range uuids {
		//csv := "CR UUID,CR name,Container UUID,State,Started At,Finished At,Duration in seconds,Compute node type,Hourly node cost,Total cost\n"

		if strings.Contains(uuid, "-d1hrv-") {
			// This is a pipeline instance, not a job! Find the cwl-runner job.
			pi := loadObject(arv, "results"+"/"+uuid, uuid)
			for _, v := range pi["components"].(map[string]interface{}) {
				x := v.(map[string]interface{})
				y := x["job"].(map[string]interface{})
				uuid = y["uuid"].(string)
			}
		}

		// for projects:
		// arv -f uuid container_request list --filters '[["owner_uuid","=","<someuuid>"],["requesting_container_uuid","=",null]]'

		// Is this a project?
		if strings.Contains(uuid, "-j7d0g-") {
			for k, v := range handleProject(uuid, arv, arv2, kc) {
				cost[k] = v
			}
		}
		// Is this a container request?
		if strings.Contains(uuid, "-xvhdp-") {
			for k, v := range generateCrCsv(uuid, arv, arv2, kc) {
				cost[k] = v
			}
		}
	}

	var csv string

	csv = "# Aggregate cost accounting for uuids:\n"
	for _, uuid := range uuids {
		csv += "# " + uuid + "\n"
	}

	var total float64
	for k, v := range cost {
		csv += k + "," + strconv.FormatFloat(v, 'f', 8, 64) + "\n"
		total += v
	}

	csv += "TOTAL," + strconv.FormatFloat(total, 'f', 8, 64) + "\n"

	// Write the resulting CSV file
	err = ioutil.WriteFile("results"+"/"+time.Now().Format("2006-01-02-15-04-05")+"-aggregate-costaccounting.csv", []byte(csv), 0644)
	if err != nil {
		logError([]string{"Error writing file", ":", err.Error()})
		os.Exit(1)
	}
}
