package core

import (
	"strings"
)

#Environments: [
	"sand",
	"dev",
	"test",
	"stage",
	"prod",
	"all",
	"global",
]

#EnvironmentPattern: strings.Join(#Environments, "|")

#EnvironmentSchema: or(#Environments)