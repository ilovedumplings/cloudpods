// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package options

import (
	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"
)

type CloudMonOptions struct {
	common_options.CommonOptions
	PingProbeOptions

	EndpointType string `default:"internalURL" help:"Defaults to internalURL" choices:"publicURL|internalURL|adminURL"`
	ApiVersion   string `help:"override default modules service api version"`

	ReqTimeout int    `default:"600" help:"Number of seconds to wait for a response"`
	Insecure   bool   `default:"true" help:"Allow skip server cert verification if URL is https" short-token:"k"`
	CertFile   string `help:"certificate file"`
	KeyFile    string `help:"private key file"`

	ResourcesSyncInterval   int64  `help:"Increment Sync Interval unit:minute" default:"10"`
	CollectMetricInterval   int64  `help:"Increment Sync Interval unit:minute" default:"6"`
	SkipMetricPullProviders string `help:"Skip indicate provider metric pull" default:""`

	InfluxDatabase string `help:"influxdb database name, default telegraf" default:"telegraf"`
}

type PingProbeOptions struct {
	Debug         bool `help:"debug"`
	ProbeCount    int  `help:"probe count, default is 3" default:"3"`
	TimeoutSecond int  `help:"probe timeout in second, default is 1 second" default:"1"`

	DisablePingProbe      bool  `help:"enable ping probe"`
	PingProbIntervalHours int64 `help:"PingProb Interval unit:hour" default:"6"`
}

var (
	Options CloudMonOptions
)

func OnOptionsChange(oldO, newO interface{}) bool {
	oldOpts := oldO.(*CloudMonOptions)
	newOpts := newO.(*CloudMonOptions)

	changed := false
	if common_options.OnCommonOptionsChange(&oldOpts.CommonOptions, &newOpts.CommonOptions) {
		changed = true
	}

	if oldOpts.DisablePingProbe != newOpts.DisablePingProbe {
		if !oldOpts.IsSlaveNode {
			changed = true
		}
	}

	return changed
}
