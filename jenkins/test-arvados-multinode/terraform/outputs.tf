# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

output "id" {
  value = module.ec2_cluster.*.id
}
output "private_dns_names" {
  value = module.ec2_cluster.*.private_dns
}
output "public_ip" {
  value = module.ec2_cluster.*.public_ip
}
output "private_ip" {
  value = module.ec2_cluster.*.private_ip
}
output "cluster_name" {
  sensitive = true
  value     = random_password.cluster_name.result
}
