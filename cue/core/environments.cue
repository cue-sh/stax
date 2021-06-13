package core

import (
	"strings"
)

#Environments: [
	"sand",
	"development",
	"dev",
	"test",
	"stage",
	"staging",
	"production",
	"prod",
	"all",
	"global",
	"citadel"
]

#EnvironmentPattern: strings.Join(#Environments, "|")

#EnvironmentSchema: or(#Environments)