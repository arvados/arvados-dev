# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

data "aws_ami" "debian" {
  most_recent = true

  filter {
    name   = "name"
    values = ["debian-11-amd64-*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  # Debian's
  owners = ["136693071363"]
}

resource "random_password" "cluster_name" {
  length  = 5
  upper   = false
  special = false
}

module "ec2_cluster" {
  source  = "terraform-aws-modules/ec2-instance/aws"
  version = "~> 3.5"

  count = var.instances_count

  name = "${var.instance_name_prefix}-${count.index}"

  ami                         = data.aws_ami.debian.id
  instance_type               = "t3.medium"
  associate_public_ip_address = true
  ebs_optimized               = true

  root_block_device = [{
    encrypted             = true,
    volume_size           = 50,
    delete_on_termination = true,
  }]

  key_name   = var.user_key
  monitoring = false
  # These are tordo's SGs
  vpc_security_group_ids = [
    "sg-07a8d44b8d75ab8de",
    "sg-0b36cbad0a62e6154",
    "sg-0fdce93c95877be0b",
    "sg-0e8fdd7632926eac6"
  ]
  subnet_id = "subnet-05b635657ce13d74e"

  tags = {
    Name        = "${var.instance_name_prefix}-${count.index}"
    Terraform   = "true"
    Environment = "dev"
    Owner       = "jenkins"
    Ticket      = var.jenkins_build_tag
    Cluster     = random_password.cluster_name.result
  }
}
