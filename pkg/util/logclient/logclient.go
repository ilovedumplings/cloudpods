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

package logclient

import (
	"context"
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/util/stringutils"
	"yunion.io/x/pkg/util/timeutils"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/appctx"
	"yunion.io/x/onecloud/pkg/appsrv"
	"yunion.io/x/onecloud/pkg/cloudcommon"
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
)

type SessionGenerator func(ctx context.Context, token mcclient.TokenCredential, region, apiVersion string) *mcclient.ClientSession

var (
	DefaultSessionGenerator = auth.GetSession
)

// golang 不支持 const 的string array, http://t.cn/EzAvbw8
var BLACK_LIST_OBJ_TYPE = []string{} // "parameter"}

var logclientWorkerMan *appsrv.SWorkerManager

func init() {
	logclientWorkerMan = appsrv.NewWorkerManager("LogClientWorkerManager", 1, 50, false)
}

type IObject interface {
	GetId() string
	GetName() string
	Keyword() string
}

type IVirtualObject interface {
	IObject
	GetOwnerId() mcclient.IIdentityProvider
}

type IModule interface {
	Create(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error)
}

// save log to db.
func AddSimpleActionLog(model IObject, action string, iNotes interface{}, userCred mcclient.TokenCredential, success bool) {
	addLog(model, action, iNotes, userCred, success, time.Time{}, &modules.Actions)
}

func AddActionLogWithContext(ctx context.Context, model IObject, action string, iNotes interface{}, userCred mcclient.TokenCredential, success bool) {
	addLog(model, action, iNotes, userCred, success, appctx.AppContextStartTime(ctx), &modules.Actions)
}

func AddActionLogWithStartable(task cloudcommon.IStartable, model IObject, action string, iNotes interface{}, userCred mcclient.TokenCredential, success bool) {
	addLog(model, action, iNotes, userCred, success, task.GetStartTime(), &modules.Actions)
}

// add websocket log to notify active browser users
func PostWebsocketNotify(model IObject, action string, iNotes interface{}, userCred mcclient.TokenCredential, success bool) {
	addLog(model, action, iNotes, userCred, success, time.Time{}, &modules.Websockets)
}

func addLog(model IObject, action string, iNotes interface{}, userCred mcclient.TokenCredential, success bool, startTime time.Time, api IModule) {
	if !consts.OpsLogEnabled() {
		return
	}
	if ok, _ := utils.InStringArray(model.Keyword(), BLACK_LIST_OBJ_TYPE); ok {
		log.Errorf("不支持的 actionlog 类型")
		return
	}
	if action == ACT_UPDATE {
		if iNotes == nil {
			return
		}
		if uds, ok := iNotes.(sqlchemy.UpdateDiffs); ok && len(uds) == 0 {
			return
		}
	}

	notes := stringutils.Interface2String(iNotes)

	objId := model.GetId()
	if len(objId) == 0 {
		objId = "-"
	}
	objName := model.GetName()
	if len(objName) == 0 {
		objName = "-"
	}

	logentry := jsonutils.NewDict()

	logentry.Add(jsonutils.NewString(objName), "obj_name")
	logentry.Add(jsonutils.NewString(model.Keyword()), "obj_type")
	logentry.Add(jsonutils.NewString(objId), "obj_id")
	logentry.Add(jsonutils.NewString(action), "action")
	logentry.Add(jsonutils.NewString(userCred.GetUserId()), "user_id")
	logentry.Add(jsonutils.NewString(userCred.GetUserName()), "user")
	logentry.Add(jsonutils.NewString(userCred.GetTenantId()), "tenant_id")
	logentry.Add(jsonutils.NewString(userCred.GetTenantName()), "tenant")
	logentry.Add(jsonutils.NewString(userCred.GetDomainId()), "domain_id")
	logentry.Add(jsonutils.NewString(userCred.GetDomainName()), "domain")
	logentry.Add(jsonutils.NewString(userCred.GetProjectDomainId()), "project_domain_id")
	logentry.Add(jsonutils.NewString(userCred.GetProjectDomain()), "project_domain")
	logentry.Add(jsonutils.NewString(strings.Join(userCred.GetRoles(), ",")), "roles")

	service := consts.GetServiceType()
	if len(service) > 0 {
		logentry.Add(jsonutils.NewString(service), "service")
	}

	if !startTime.IsZero() {
		logentry.Add(jsonutils.NewString(timeutils.FullIsoTime(startTime)), "start_time")
	}

	if virtualModel, ok := model.(IVirtualObject); ok {
		ownerId := virtualModel.GetOwnerId()
		if ownerId != nil {
			projectId := ownerId.GetProjectId()
			if len(projectId) > 0 {
				logentry.Add(jsonutils.NewString(projectId), "owner_tenant_id")
			}
			domainId := ownerId.GetProjectDomainId()
			if len(domainId) > 0 {
				logentry.Add(jsonutils.NewString(domainId), "owner_domain_id")
			}
		}
	}

	if !success {
		// 失败日志
		logentry.Add(jsonutils.JSONFalse, "success")
	} else {
		// 成功日志
		logentry.Add(jsonutils.JSONTrue, "success")
	}

	logentry.Add(jsonutils.NewString(notes), "notes")

	logclientWorkerMan.Run(func() {
		s := DefaultSessionGenerator(context.Background(), userCred, "", "")
		_, err := api.Create(s, logentry)
		if err != nil {
			log.Errorf("create action log %s failed %s", logentry, err)
		}
	}, nil, nil)
}
