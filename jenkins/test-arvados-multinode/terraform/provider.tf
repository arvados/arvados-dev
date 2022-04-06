# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: Apache-2.0

provider "aws" {
  region  = "us-east-1"
}

terraform {
  required_providers {
    aws = {
      version = "~> 4.8.0"
    }
  }
}
