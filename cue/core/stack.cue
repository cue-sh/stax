package core

import (
	aws "github.com/cue-sh/cfn-cue/aws/useast1"
)

#Stack: {
	DependsOn?: [...string]
	Environment: #EnvironmentSchema
	Name:     string
	Overrides?:{
		[string]: {
			SopsProfile?: string
			Map?: {...}
		}
	}
	Params?: {}
	Profile:     string
	Region:      #RegionSchema
	RegionCode?:  string
	Role?: =~"^arn:aws:iam::\\d{12}:role/[a-zA-Z0-9\\-_+=,.@]{1,64}$"
	Template: aws.#Template
	Template: AWSTemplateFormatVersion: _
	Tags?: [string]: string
	TagsEnabled: *true | false
}