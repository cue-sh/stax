package core

import (
  "strings"
)

#RegionCodes: {
  usw2: "us-west-2"
  usw1: "us-west-1"
  use2: "us-east-2"
  use1: "us-east-1"
  afs1: "af-south-1"
  ape1: "ap-east-1"
  aps1: "ap-south-1"
  apne1: "ap-northeast-1"
  apne2: "ap-northeast-2"
  apne3: "ap-northeast-3"
  apse1: "ap-southeast-1"
  apse2: "ap-southeast-2"
  cac1: "ca-central-1"
  euc1: "eu-central-1"
  euw1: "eu-west-1"
  euw2: "eu-west-2"
  eus1: "eu-south-1"
  euw3: "eu-west-3"
  eun1: "eu-north-1"
  mes1: "me-south-1"
  sae1: "sa-east-1"
}

#RegionCodePattern: strings.Join([ for regionCode, region in #RegionCodes { regionCode }], "|")

#RegionSchema: or([ for regionCode, region in #RegionCodes { region } ])