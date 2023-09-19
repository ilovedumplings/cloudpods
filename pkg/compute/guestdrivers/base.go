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

package guestdrivers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"yunion.io/x/cloudmux/pkg/cloudprovider"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/billing"
	"yunion.io/x/pkg/util/osprofile"

	"yunion.io/x/onecloud/pkg/apis"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/quotas"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	guestdriver_types "yunion.io/x/onecloud/pkg/compute/guestdrivers/types"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/compute/options"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
)

type SBaseGuestScheduleDriver struct{}

func (d SBaseGuestScheduleDriver) DoScheduleSKUFilter() bool { return true }

func (d SBaseGuestScheduleDriver) DoScheduleCPUFilter() bool { return true }

func (d SBaseGuestScheduleDriver) DoScheduleMemoryFilter() bool { return true }

func (d SBaseGuestScheduleDriver) DoScheduleStorageFilter() bool { return true }

func (d SBaseGuestScheduleDriver) DoScheduleCloudproviderTagFilter() bool { return false }

type SBaseGuestDriver struct {
	SBaseGuestScheduleDriver
}

func (drv *SBaseGuestDriver) IsAllowSaveImageOnRunning() bool {
	return false
}

func (drv *SBaseGuestDriver) StartGuestCreateTask(guest *models.SGuest, ctx context.Context, userCred mcclient.TokenCredential, data *jsonutils.JSONDict, pendingUsage quotas.IQuota, parentTaskId string) error {
	task, err := taskman.TaskManager.NewTask(ctx, "GuestCreateTask", guest, userCred, data, parentTaskId, "", pendingUsage)
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) OnGuestCreateTaskComplete(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	duration, _ := task.GetParams().GetString("duration")
	if len(duration) > 0 {
		bc, err := billing.ParseBillingCycle(duration)
		if err == nil && guest.ExpiredAt.IsZero() {
			guest.SaveRenewInfo(ctx, task.GetUserCred(), &bc, nil, "")
		}
		if jsonutils.QueryBoolean(task.GetParams(), "auto_prepaid_recycle", false) {
			err := guest.CanPerformPrepaidRecycle()
			if err == nil {
				task.SetStageComplete(ctx, nil)
				guest.DoPerformPrepaidRecycle(ctx, task.GetUserCred(), true)
				return nil
			}
		}
	}
	if jsonutils.QueryBoolean(task.GetParams(), "auto_start", false) {
		task.SetStage("on_auto_start_guest", nil)
		return guest.StartGueststartTask(ctx, task.GetUserCred(), nil, task.GetTaskId())
	} else {
		task.SetStage("on_sync_status_complete", nil)
		return guest.StartSyncstatus(ctx, task.GetUserCred(), task.GetTaskId())
	}
}

func (drv *SBaseGuestDriver) StartDeleteGuestTask(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, params *jsonutils.JSONDict, parentTaskId string) error {
	task, err := taskman.TaskManager.NewTask(ctx, "GuestDeleteTask", guest, userCred, params, parentTaskId, "", nil)
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) ValidateImage(ctx context.Context, image *cloudprovider.SImage) error {
	return nil
}

func (drv *SBaseGuestDriver) RequestDetachDisksFromGuestForDelete(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) OnDeleteGuestFinalCleanup(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential) error {
	return guest.DeleteAllDisksInDB(ctx, userCred)
}

func (drv *SBaseGuestDriver) RequestDetachDisk(ctx context.Context, guest *models.SGuest, disk *models.SDisk, task taskman.ITask) error {
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) RequestAttachDisk(ctx context.Context, guest *models.SGuest, disk *models.SDisk, task taskman.ITask) error {
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) RequestOpenForward(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, req *guestdriver_types.OpenForwardRequest) (*guestdriver_types.OpenForwardResponse, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestListForward(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, req *guestdriver_types.ListForwardRequest) (*guestdriver_types.ListForwardResponse, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestCloseForward(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, req *guestdriver_types.CloseForwardRequest) (*guestdriver_types.CloseForwardResponse, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestSaveImage(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, task taskman.ITask) error {
	return errors.Wrapf(cloudprovider.ErrNotImplemented, "RequestSaveImage")
}

func (drv *SBaseGuestDriver) RequestGuestCreateAllDisks(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) GetDetachDiskStatus() ([]string, error) {
	return []string{}, fmt.Errorf("This Guest driver dose not implement GetDetachDiskStatus")
}

func (drv *SBaseGuestDriver) GetAttachDiskStatus() ([]string, error) {
	return []string{}, fmt.Errorf("This Guest driver dose not implement GetAttachDiskStatus")
}

func (drv *SBaseGuestDriver) GetRebuildRootStatus() ([]string, error) {
	return []string{}, fmt.Errorf("This Guest driver dose not implement GetRebuildRootStatus")
}

func (drv *SBaseGuestDriver) IsRebuildRootSupportChangeImage() bool {
	return true
}

func (drv *SBaseGuestDriver) IsRebuildRootSupportChangeUEFI() bool {
	return true
}

func (drv *SBaseGuestDriver) GetChangeConfigStatus(guest *models.SGuest) ([]string, error) {
	return []string{}, fmt.Errorf("This Guest driver dose not implement GetChangeConfigStatus")
}

func (drv *SBaseGuestDriver) ValidateChangeConfig(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, cpuChanged bool, memChanged bool, newDisks []*api.DiskConfig) error {
	return nil
}

func (drv *SBaseGuestDriver) ValidateDetachDisk(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, disk *models.SDisk) error {
	return nil
}

func (drv *SBaseGuestDriver) ValidateCreateEip(ctx context.Context, userCred mcclient.TokenCredential, input api.ServerCreateEipInput) error {
	return httperrors.NewInputParameterError("Not Implement ValidateCreateEip")
}

func (drv *SBaseGuestDriver) ValidateResizeDisk(guest *models.SGuest, disk *models.SDisk, storage *models.SStorage) error {
	return fmt.Errorf("This Guest driver dose not implement ValidateResizeDisk")
}

func (drv *SBaseGuestDriver) GetDeployStatus() ([]string, error) {
	return []string{}, fmt.Errorf("This Guest driver dose not implement GetDeployStatus")
}

func (drv *SBaseGuestDriver) IsNeedRestartForResetLoginInfo() bool {
	return true
}

func (drv *SBaseGuestDriver) RequestDeleteDetachedDisk(ctx context.Context, disk *models.SDisk, task taskman.ITask, isPurge bool) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestSuspendOnHost(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestResumeOnHost(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) StartGuestResetTask(guest *models.SGuest, ctx context.Context, userCred mcclient.TokenCredential, isHard bool, parentTaskId string) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) StartGuestRestartTask(guest *models.SGuest, ctx context.Context, userCred mcclient.TokenCredential, isForce bool, parentTaskId string) error {
	data := jsonutils.NewDict()
	data.Set("is_force", jsonutils.NewBool(isForce))
	if err := guest.SetStatus(userCred, api.VM_STOPPING, ""); err != nil {
		return err
	}
	task, err := taskman.TaskManager.NewTask(ctx, "GuestRestartTask", guest, userCred, nil, parentTaskId, "", nil)
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) RequestSoftReset(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) AllowReconfigGuest() bool {
	return true
}

func (drv *SBaseGuestDriver) DoGuestCreateDisksTask(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestChangeVmConfig(ctx context.Context, guest *models.SGuest, task taskman.ITask, instanceType string, vcpuCount, vmemSize int64) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) NeedRequestGuestHotAddIso(ctx context.Context, guest *models.SGuest) bool {
	return false
}

func (drv *SBaseGuestDriver) RequestGuestHotAddIso(ctx context.Context, guest *models.SGuest, path string, boot bool, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestGuestHotRemoveIso(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestRebuildRootDisk(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) NeedRequestGuestHotAddVfd(ctx context.Context, guest *models.SGuest) bool {
	return false
}

func (drv *SBaseGuestDriver) RequestGuestHotAddVfd(ctx context.Context, guest *models.SGuest, path string, boot bool, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestGuestHotRemoveVfd(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestDiskSnapshot(ctx context.Context, guest *models.SGuest, task taskman.ITask, snapshotId, diskId string) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestDeleteSnapshot(ctx context.Context, guest *models.SGuest, task taskman.ITask, params *jsonutils.JSONDict) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestReloadDiskSnapshot(ctx context.Context, guest *models.SGuest, task taskman.ITask, params *jsonutils.JSONDict) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestSyncToBackup(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) RequestSlaveBlockStreamDisks(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement")
}

func (drv *SBaseGuestDriver) GetMaxSecurityGroupCount() int {
	return 5
}

func (drv *SBaseGuestDriver) getTaskRequestHeader(task taskman.ITask) http.Header {
	return task.GetTaskRequestHeader()
}

func (drv *SBaseGuestDriver) IsSupportedBillingCycle(bc billing.SBillingCycle) bool {
	return true
}

func (drv *SBaseGuestDriver) IsSupportPostpaidExpire() bool {
	return true
}

func (drv *SBaseGuestDriver) IsSupportShutdownMode() bool {
	return false
}

func (drv *SBaseGuestDriver) RequestRenewInstance(ctx context.Context, guest *models.SGuest, bc billing.SBillingCycle) (time.Time, error) {
	return time.Time{}, nil
}

func (drv *SBaseGuestDriver) IsSupportEip() bool {
	return false
}

func (drv *SBaseGuestDriver) IsSupportPublicIp() bool {
	return false
}

func (drv *SBaseGuestDriver) NeedStopForChangeSpec(ctx context.Context, guest *models.SGuest, cpuChanged, memChanged bool) bool {
	return false
}

func (drv *SBaseGuestDriver) RemoteDeployGuestForCreate(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, host *models.SHost, desc cloudprovider.SManagedVMCreateConfig) (jsonutils.JSONObject, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (drv *SBaseGuestDriver) RemoteDeployGuestSyncHost(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, host *models.SHost, iVM cloudprovider.ICloudVM) (cloudprovider.ICloudHost, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (drv *SBaseGuestDriver) RemoteActionAfterGuestCreated(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, host *models.SHost, ivm cloudprovider.ICloudVM, desc *cloudprovider.SManagedVMCreateConfig) {
	return
}

func (drv *SBaseGuestDriver) RemoteDeployGuestForDeploy(ctx context.Context, guest *models.SGuest, ihost cloudprovider.ICloudHost, task taskman.ITask, desc cloudprovider.SManagedVMCreateConfig) (jsonutils.JSONObject, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (drv *SBaseGuestDriver) RemoteDeployGuestForRebuildRoot(ctx context.Context, guest *models.SGuest, ihost cloudprovider.ICloudHost, task taskman.ITask, desc cloudprovider.SManagedVMCreateConfig) (jsonutils.JSONObject, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (drv *SBaseGuestDriver) GetGuestInitialStateAfterCreate() string {
	return api.VM_READY
}

func (drv *SBaseGuestDriver) GetGuestInitialStateAfterRebuild() string {
	return api.VM_READY
}

func (drv *SBaseGuestDriver) IsNeedInjectPasswordByCloudInit() bool {
	return false
}

func (drv *SBaseGuestDriver) GetWindowsUserDataType() string {
	return cloudprovider.CLOUD_POWER_SHELL
}

func (drv *SBaseGuestDriver) IsWindowsUserDataTypeNeedEncode() bool {
	return false
}

func (drv *SBaseGuestDriver) IsSupportdDcryptPasswordFromSecretKey() bool {
	return true
}

func (drv *SBaseGuestDriver) GetUserDataType() string {
	return cloudprovider.CLOUD_CONFIG
}

func (drv *SBaseGuestDriver) GetDefaultAccount(osType, osDist, imageType string) string {
	if strings.ToLower(osType) == strings.ToLower(osprofile.OS_TYPE_WINDOWS) {
		return api.VM_DEFAULT_WINDOWS_LOGIN_USER
	}
	return api.VM_DEFAULT_LINUX_LOGIN_USER
}

func (drv *SBaseGuestDriver) OnGuestChangeCpuMemFailed(ctx context.Context, guest *models.SGuest, data *jsonutils.JSONDict, task taskman.ITask) error {
	return nil
}

func (drv *SBaseGuestDriver) RequestSyncConfigOnHost(ctx context.Context, guest *models.SGuest, host *models.SHost, task taskman.ITask) error {
	return fmt.Errorf("SBaseGuestDriver: Not Implement")
}

func (drv *SBaseGuestDriver) IsSupportGuestClone() bool {
	return true
}

func (drv *SBaseGuestDriver) RequestSyncSecgroupsOnHost(ctx context.Context, guest *models.SGuest, host *models.SHost, task taskman.ITask) error {
	return nil // do nothing
}

func (drv *SBaseGuestDriver) CancelExpireTime(
	ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest) error {
	return guest.CancelExpireTime(ctx, userCred)
}

func (drv *SBaseGuestDriver) IsSupportPublicipToEip() bool {
	return false
}

func (drv *SBaseGuestDriver) RequestConvertPublicipToEip(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, task taskman.ITask) error {
	return fmt.Errorf("Not Implement RequestConvertPublicipToEip")
}

func (drv *SBaseGuestDriver) IsSupportSetAutoRenew() bool {
	return false
}

func (drv *SBaseGuestDriver) RequestSetAutoRenewInstance(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, input api.GuestAutoRenewInput, task taskman.ITask) error {
	return fmt.Errorf("Not Implement RequestSetAutoRenewInstance")
}

func (drv *SBaseGuestDriver) IsSupportMigrate() bool {
	return false
}

func (drv *SBaseGuestDriver) IsSupportLiveMigrate() bool {
	return false
}

func (drv *SBaseGuestDriver) CheckMigrate(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential, input api.GuestMigrateInput) error {
	return httperrors.NewNotAcceptableError("Not allow for hypervisor %s", guest.GetHypervisor())
}

func (drv *SBaseGuestDriver) CheckLiveMigrate(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential, input api.GuestLiveMigrateInput) error {
	return httperrors.NewNotAcceptableError("Not allow for hypervisor %s", guest.GetHypervisor())
}

func (drv *SBaseGuestDriver) RequestMigrate(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential, input api.GuestMigrateInput, task taskman.ITask) error {
	return errors.Wrapf(cloudprovider.ErrNotImplemented, "RequestMigrate")
}

func (drv *SBaseGuestDriver) RequestLiveMigrate(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential, input api.GuestLiveMigrateInput, task taskman.ITask) error {
	return errors.Wrapf(cloudprovider.ErrNotImplemented, "RequestLiveMigrate")
}

func (drv *SBaseGuestDriver) RequestCancelLiveMigrate(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential) error {
	return errors.Wrapf(cloudprovider.ErrNotImplemented, "RequestCancelLiveMigrate")
}

func (drv *SVirtualizedGuestDriver) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, input *api.ServerCreateInput) (*api.ServerCreateInput, error) {
	return input, nil
}

func (drv *SBaseGuestDriver) ValidateUpdateData(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential, input api.ServerUpdateInput) (api.ServerUpdateInput, error) {
	return input, nil
}

func (drv *SBaseGuestDriver) RequestRemoteUpdate(ctx context.Context, guest *models.SGuest, userCred mcclient.TokenCredential, replaceTags bool) error {
	// nil ops
	return nil
}

func (drv *SBaseGuestDriver) ValidateRebuildRoot(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, input *api.ServerRebuildRootInput) (*api.ServerRebuildRootInput, error) {
	return input, nil
}

func (drv *SBaseGuestDriver) ValidateDetachNetwork(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest) error {
	return nil
}

func (drv *SBaseGuestDriver) ValidateChangeDiskStorage(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, targetStorageId string) error {
	return cloudprovider.ErrNotImplemented
}

func (drv *SBaseGuestDriver) StartChangeDiskStorageTask(guest *models.SGuest, ctx context.Context, userCred mcclient.TokenCredential, params *api.ServerChangeDiskStorageInternalInput, parentTaskId string) error {
	task, err := taskman.TaskManager.NewTask(ctx, "GuestChangeDiskStorageTask", guest, userCred, jsonutils.Marshal(params).(*jsonutils.JSONDict), parentTaskId, "", nil)
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) RequestChangeDiskStorage(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, input *api.ServerChangeDiskStorageInternalInput, task taskman.ITask) error {
	return cloudprovider.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestSwitchToTargetStorageDisk(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, input *api.ServerChangeDiskStorageInternalInput, task taskman.ITask) error {
	return cloudprovider.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestSyncIsolatedDevice(ctx context.Context, guest *models.SGuest, task taskman.ITask) error {
	task.ScheduleRun(nil)
	return nil
}

func (drv *SBaseGuestDriver) RequestCPUSet(ctx context.Context, userCred mcclient.TokenCredential, host *models.SHost, guest *models.SGuest, input *api.ServerCPUSetInput) (*api.ServerCPUSetResp, error) {
	return nil, httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestCPUSetRemove(ctx context.Context, userCred mcclient.TokenCredential, host *models.SHost, guest *models.SGuest, input *api.ServerCPUSetRemoveInput) error {
	return httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) QgaRequestGuestPing(ctx context.Context, header http.Header, host *models.SHost, guest *models.SGuest, async bool, input *api.ServerQgaTimeoutInput) error {
	return httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) QgaRequestSetUserPassword(ctx context.Context, task taskman.ITask, host *models.SHost, guest *models.SGuest, input *api.ServerQgaSetPasswordInput) error {
	return httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestQgaCommand(ctx context.Context, userCred mcclient.TokenCredential, body jsonutils.JSONObject, host *models.SHost, guest *models.SGuest) (jsonutils.JSONObject, error) {
	return nil, httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) FetchMonitorUrl(ctx context.Context, guest *models.SGuest) string {
	s := auth.GetAdminSessionWithPublic(ctx, consts.GetRegion())
	influxdbUrl, err := s.GetServiceURL(apis.SERVICE_TYPE_INFLUXDB, options.Options.MonitorEndpointType)
	if err != nil {
		log.Errorf("FetchMonitorUrl fail %s", err)
	}
	return influxdbUrl
}

func (drv *SBaseGuestDriver) RequestResetNicTrafficLimit(ctx context.Context, task taskman.ITask, host *models.SHost, guest *models.SGuest, input *api.ServerNicTrafficLimit) error {
	return httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) RequestSetNicTrafficLimit(ctx context.Context, task taskman.ITask, host *models.SHost, guest *models.SGuest, input *api.ServerNicTrafficLimit) error {
	return httperrors.ErrNotImplemented
}

func (drv *SBaseGuestDriver) SyncOsInfo(ctx context.Context, userCred mcclient.TokenCredential, g *models.SGuest, extVM cloudprovider.IOSInfo) error {
	return nil
}
