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

package tasks

import (
	"context"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/notifyclient"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/logclient"
)

type LoadbalancerCertificateDeleteTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(LoadbalancerCertificateDeleteTask{})
}

func (self *LoadbalancerCertificateDeleteTask) taskFail(ctx context.Context, lbcert *models.SCachedLoadbalancerCertificate, err error) {
	lbcert.SetStatus(ctx, self.GetUserCred(), api.LB_STATUS_DELETE_FAILED, err.Error())
	db.OpsLog.LogEvent(lbcert, db.ACT_DELOCATE_FAIL, err, self.UserCred)
	logclient.AddActionLogWithStartable(self, lbcert, logclient.ACT_DELOCATE, err, self.UserCred, false)
	notifyclient.NotifySystemErrorWithCtx(ctx, lbcert.Id, lbcert.Name, api.LB_STATUS_DELETE_FAILED, err.Error())
	self.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (self *LoadbalancerCertificateDeleteTask) OnInit(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	lbcert := obj.(*models.SCachedLoadbalancerCertificate)
	region, err := lbcert.GetRegion()
	if err != nil {
		self.taskFail(ctx, lbcert, errors.Wrapf(err, "GetRegion"))
		return
	}
	self.SetStage("OnLoadbalancerCertificateDeleteComplete", nil)
	err = region.GetDriver().RequestDeleteLoadbalancerCertificate(ctx, self.GetUserCred(), lbcert, self)
	if err != nil {
		self.taskFail(ctx, lbcert, errors.Wrapf(err, "RequestDeleteLoadbalancerCertificate"))
	}
}

func (self *LoadbalancerCertificateDeleteTask) OnLoadbalancerCertificateDeleteComplete(ctx context.Context, lbcert *models.SCachedLoadbalancerCertificate, data jsonutils.JSONObject) {
	db.OpsLog.LogEvent(lbcert, db.ACT_DELETE, lbcert.GetShortDesc(ctx), self.UserCred)
	logclient.AddActionLogWithStartable(self, lbcert, logclient.ACT_DELOCATE, nil, self.UserCred, true)
	lbcert.RealDelete(ctx, self.GetUserCred())
	self.SetStageComplete(ctx, nil)
}

func (self *LoadbalancerCertificateDeleteTask) OnLoadbalancerCertificateDeleteCompleteFailed(ctx context.Context, lbcert *models.SCachedLoadbalancerCertificate, reason jsonutils.JSONObject) {
	self.taskFail(ctx, lbcert, errors.Errorf(reason.String()))
}
