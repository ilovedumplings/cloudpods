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

package service

import (
	"os"
	"time"

	"yunion.io/x/log"

	"yunion.io/x/onecloud/pkg/apis"
	app_common "yunion.io/x/onecloud/pkg/cloudcommon/app"
	"yunion.io/x/onecloud/pkg/cloudcommon/cronman"
	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/cloudmon/misc"
	"yunion.io/x/onecloud/pkg/cloudmon/options"
	"yunion.io/x/onecloud/pkg/cloudmon/resources"
	_ "yunion.io/x/onecloud/pkg/multicloud/loader"
)

func StartService() {
	opts := &options.Options
	baseOpts := &options.Options.BaseOptions
	common_options.ParseOptions(opts, os.Args, "cloudmon.conf", "cloudmon")

	commonOpts := &opts.CommonOptions
	app_common.InitAuth(commonOpts, func() {
		log.Infof("Auth complete")
	})
	common_options.StartOptionManager(opts, opts.ConfigSyncPeriodSeconds, apis.SERVICE_TYPE_CLOUDMON, "", options.OnOptionsChange)

	res := resources.NewResources()
	if !opts.IsSlaveNode {
		cron := cronman.InitCronJobManager(true, options.Options.CronJobWorkerCount)
		cron.AddJobAtIntervalsWithStartRun("InitResources", time.Duration(opts.ResourcesSyncInterval)*time.Minute, res.Init, true)
		cron.AddJobAtIntervals("IncrementResources", time.Duration(opts.ResourcesSyncInterval)*time.Minute, res.IncrementSync)
		cron.AddJobAtIntervals("DecrementResources", time.Duration(opts.ResourcesSyncInterval)*time.Minute, res.DecrementSync)
		cron.AddJobAtIntervals("UpdateResources", time.Duration(opts.ResourcesSyncInterval)*time.Minute, res.UpdateSync)
		cron.AddJobAtIntervalsWithStarTime("CollectResources", time.Duration(opts.CollectMetricInterval)*time.Minute, res.CollectMetrics)

		cron.AddJobAtIntervals("PingProb", time.Duration(opts.PingProbIntervalHours)*time.Hour, misc.PingProbe)

		cron.AddJobEveryFewDays("UsageMetricCollect", 1, 23, 10, 10, misc.UsegReport, false)
		cron.AddJobEveryFewDays("AlertHistoryMetricCollect", 1, 23, 59, 59, misc.AlertHistoryReport, false)
		go cron.Start()
	}

	app := app_common.InitApp(baseOpts, true)
	app_common.ServeForever(app, baseOpts)
}
