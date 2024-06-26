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

package compute

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/mcclient"
	modules "yunion.io/x/onecloud/pkg/mcclient/modules/compute"
	options "yunion.io/x/onecloud/pkg/mcclient/options/compute"
)

func init() {
	R(&options.PodCreateOptions{}, "pod-create", "Create a container pod", func(s *mcclient.ClientSession, opts *options.PodCreateOptions) error {
		params, err := opts.Params()
		if err != nil {
			return err
		}
		if opts.Count > 1 {
			results := modules.Servers.BatchCreate(s, params.JSON(params), opts.Count)
			printBatchResults(results, modules.Servers.GetColumns(s))
		} else {
			server, err := modules.Servers.Create(s, params.JSON(params))
			if err != nil {
				return err
			}
			printObject(server)
		}
		return nil
	})

	R(&options.PodExecOptions{}, "pod-exec", "Execute a command in a container", func(s *mcclient.ClientSession, opt *options.PodExecOptions) error {
		ctrs, err := modules.Containers.List(s, jsonutils.Marshal(map[string]string{
			"guest_id": opt.ID,
		}))
		if err != nil {
			return errors.Wrapf(err, "list containers by guest_id %s", opt.ID)
		}
		if len(ctrs.Data) == 0 {
			return errors.Errorf("count of container is 0")
		}
		var ctrId string
		if opt.Container == "" {
			ctrId, _ = ctrs.Data[0].GetString("id")
		} else {
			for _, ctr := range ctrs.Data {
				id, _ := ctr.GetString("id")
				name, _ := ctr.GetString("name")
				if opt.Container == id || opt.Container == name {
					ctrId, _ = ctr.GetString("id")
					break
				}
			}
		}
		return modules.Containers.Exec(s, ctrId, opt.ToAPIInput())
	})
}
