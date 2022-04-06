# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

variable "jenkins_build_tag" {
  type = string
}
variable "instance_name_prefix" {
  type = string
}
variable "instances_count" {
  type    = number
  default = 2
}
variable "user_key" {
  type = string
}
