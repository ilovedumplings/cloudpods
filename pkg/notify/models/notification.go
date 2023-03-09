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

package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/rbacscope"
	"yunion.io/x/pkg/util/sets"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/notify"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/validators"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/notify/options"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SNotificationManager struct {
	db.SStatusStandaloneResourceBaseManager
}

var NotificationManager *SNotificationManager

func init() {
	NotificationManager = &SNotificationManager{
		SStatusStandaloneResourceBaseManager: db.NewStatusStandaloneResourceBaseManager(
			SNotification{},
			"notifications_tbl",
			"notification",
			"notifications",
		),
	}
	NotificationManager.SetVirtualObject(NotificationManager)
	NotificationManager.TableSpec().AddIndex(false, "deleted", "contact_type", "topic_type")
}

// 站内信
type SNotification struct {
	db.SStatusStandaloneResourceBase

	ContactType string `width:"128" nullable:"true" create:"optional" list:"user" get:"user"`
	// swagger:ignore
	Topic    string `width:"128" nullable:"true" create:"required" list:"user" get:"user"`
	Priority string `width:"16" nullable:"true" create:"optional" list:"user" get:"user"`
	// swagger:ignore
	Message string `create:"required"`
	// swagger:ignore
	TopicType  string    `json:"topic_type" width:"20" nullable:"true" update:"user" list:"user"`
	ReceivedAt time.Time `nullable:"true" list:"user" get:"user"`
	EventId    string    `width:"128" nullable:"true"`

	SendTimes int
}

const (
	SendByContact = "send_by_contact"
)

func (nm *SNotificationManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.NotificationCreateInput) (api.NotificationCreateInput, error) {
	cTypes := []string{}
	if len(input.Contacts) > 0 && !userCred.HasSystemAdminPrivilege() {
		return input, httperrors.NewForbiddenError("only admin can send notification by contact")
	}

	// check robot
	robots := []string{}
	for i := range input.Robots {
		_robot, err := validators.ValidateModel(userCred, RobotManager, &input.Robots[i])
		if err != nil && !input.IgnoreNonexistentReceiver {
			return input, err
		}
		if _robot != nil {
			robot := _robot.(*SRobot)
			if !utils.IsInStringArray(robot.GetId(), robots) {
				robots = append(robots, robot.GetId())
			}
			if !utils.IsInStringArray(robot.Type, cTypes) {
				cTypes = append(cTypes, robot.Type)
			}
		}
	}
	input.Robots = robots

	// check receivers
	receivers, err := ReceiverManager.FetchByIdOrNames(ctx, input.Receivers...)
	if err != nil {
		return input, errors.Wrap(err, "ReceiverManager.FetchByIDs")
	}
	idSet := sets.NewString()
	nameSet := sets.NewString()
	for i := range receivers {
		idSet.Insert(receivers[i].Id)
		nameSet.Insert(receivers[i].Name)
	}
	for _, re := range input.Receivers {
		if idSet.Has(re) || nameSet.Has(re) {
			continue
		}
		if input.ContactType == api.WEBCONSOLE {
			input.Contacts = append(input.Contacts, re)
		}
		if !input.IgnoreNonexistentReceiver {
			return input, httperrors.NewInputParameterError("no such receiver whose uid is %q", re)
		}
	}
	input.Receivers = idSet.UnsortedList()
	if len(input.Receivers)+len(input.Contacts) == 0 {
		return input, httperrors.NewInputParameterError("no valid receiver or contact")
	}

	if len(input.Receivers)+len(input.Contacts)+len(input.Robots) == 0 {
		return input, httperrors.NewInputParameterError("no valid receiver or contact")
	}
	input.ContactType = strings.Join(cTypes, ",")
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	if len(input.Priority) == 0 {
		input.Priority = api.NOTIFICATION_PRIORITY_NORMAL
	}
	// hack
	length := 10
	topicRunes := []rune(input.Topic)
	if len(topicRunes) < 10 {
		length = len(topicRunes)
	}
	name := fmt.Sprintf("%s-%s-%s", string(topicRunes[:length]), input.ContactType, nowStr)
	input.Name, err = db.GenerateName(ctx, nm, ownerId, name)
	if err != nil {
		return input, errors.Wrapf(err, "unable to generate name for %s", name)
	}
	return input, nil
}

func (n *SNotification) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	n.ReceivedAt = time.Now()
	n.Id = db.DefaultUUIDGenerator()
	var input api.NotificationCreateInput
	err := data.Unmarshal(&input)
	if err != nil {
		return err
	}
	for i := range input.Receivers {
		_, err := ReceiverNotificationManager.Create(ctx, userCred, input.Receivers[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.Create")
		}
	}
	for i := range input.Contacts {
		_, err := ReceiverNotificationManager.CreateContact(ctx, userCred, input.Contacts[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.CreateContact")
		}
	}
	for i := range input.Robots {
		_, err := ReceiverNotificationManager.CreateRobot(ctx, userCred, input.Robots[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.CreateRobot")
		}
	}
	return nil
}

func (n *SNotification) PostCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	n.SStatusStandaloneResourceBase.PostCreate(ctx, userCred, ownerId, query, data)
	n.SetStatus(userCred, api.NOTIFICATION_STATUS_RECEIVED, "")
	task, err := taskman.TaskManager.NewTask(ctx, "NotificationSendTask", n, userCred, nil, "", "")
	if err != nil {
		n.SetStatus(userCred, api.NOTIFICATION_STATUS_FAILED, "NewTask")
		return
	}
	task.ScheduleRun(nil)
}

// TODO: support project and domain
func (nm *SNotificationManager) PerformEventNotify(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.NotificationManagerEventNotifyInput) (api.NotificationManagerEventNotifyOutput, error) {
	var output api.NotificationManagerEventNotifyOutput
	// contact type
	contactTypes := input.ContactTypes
	cts, err := ConfigManager.allContactType()
	if err != nil {
		return output, errors.Wrap(err, "unable to fetch allContactType")
	}
	if len(contactTypes) == 0 {
		contactTypes = append(contactTypes, cts...)
	}

	topic, err := TopicManager.TopicByEvent(input.Event, input.AdvanceDays)
	if err != nil {
		return output, errors.Wrapf(err, "unable fetch subscriptions by event %q", input.Event)
	}
	if topic == nil {
		return output, nil
	}
	var receiverIds []string
	receiverIds1, err := SubscriberManager.getReceiversSent(ctx, topic.Id, input.ProjectDomainId, input.ProjectId)
	if err != nil {
		return output, errors.Wrap(err, "unable to get receive")
	}
	receiverIds = append(receiverIds, receiverIds1...)

	// robot
	var robots []string
	_robots, err := SubscriberManager.robot(topic.Id, input.ProjectDomainId, input.ProjectId)
	if err != nil {
		if errors.Cause(err) != errors.ErrNotFound {
			return output, errors.Wrapf(err, "unable fetch robot of subscription %q", topic.Id)
		}
	} else {
		robots = append(robots, _robots...)
	}

	var webhookRobots []string
	if len(robots) > 0 {
		robots = sets.NewString(robots...).UnsortedList()
		rs, err := RobotManager.FetchByIdOrNames(ctx, robots...)
		if err != nil {
			return output, errors.Wrap(err, "unable to get robots")
		}
		robots, webhookRobots = make([]string, 0, len(rs)), make([]string, 0, 1)
		for i := range rs {
			if rs[i].Type == api.ROBOT_TYPE_WEBHOOK {
				webhookRobots = append(webhookRobots, rs[i].Id)
			} else {
				robots = append(robots, rs[i].Id)
			}
		}
	}

	message := jsonutils.Marshal(input.ResourceDetails).String()

	// append default receiver
	receiverIds = append(receiverIds, input.ReceiverIds...)
	// fillter non-existed receiver
	receivers, err := ReceiverManager.FetchByIdOrNames(ctx, receiverIds...)
	if err != nil {
		return output, errors.Wrap(err, "unable to fetch receivers by ids")
	}
	webconsoleContacts := sets.NewString()
	idSet := sets.NewString()
	for i := range receivers {
		idSet.Insert(receivers[i].Id)
	}
	for _, re := range receiverIds {
		if idSet.Has(re) {
			continue
		}
		webconsoleContacts.Insert(re)
	}
	receiverIds = idSet.UnsortedList()

	// create event
	event, err := EventManager.CreateEvent(ctx, input.Event, topic.Id, message, string(input.Action), input.ResourceType, input.AdvanceDays)
	if err != nil {
		return output, errors.Wrap(err, "unable to create Event")
	}

	if nm.needWebconsole([]STopic{*topic}) {
		// webconsole
		err = nm.create(ctx, userCred, api.WEBCONSOLE, receiverIds, webconsoleContacts.UnsortedList(), input.Priority, event.GetId(), topic.Type)
		if err != nil {
			output.FailedList = append(output.FailedList, api.FailedElem{
				ContactType: api.WEBCONSOLE,
				Reason:      err.Error(),
			})
		}
	}
	// normal contact type
	for _, ct := range contactTypes {
		if ct == api.MOBILE {
			continue
		}
		err := nm.create(ctx, userCred, ct, receiverIds, nil, input.Priority, event.GetId(), topic.Type)
		if err != nil {
			output.FailedList = append(output.FailedList, api.FailedElem{
				ContactType: ct,
				Reason:      err.Error(),
			})
		}
	}
	err = nm.createWithWebhookRobots(ctx, userCred, webhookRobots, input.Priority, event.GetId(), topic.Type)
	if err != nil {
		output.FailedList = append(output.FailedList, api.FailedElem{
			ContactType: api.WEBHOOK,
			Reason:      err.Error(),
		})
	}
	// robot
	err = nm.createWithRobots(ctx, userCred, robots, input.Priority, event.GetId(), topic.Type)
	if err != nil {
		output.FailedList = append(output.FailedList, api.FailedElem{
			ContactType: api.ROBOT,
			Reason:      err.Error(),
		})
	}
	return output, nil
}

func (nm *SNotificationManager) needWebconsole(topics []STopic) bool {
	for i := range topics {
		if topics[i].WebconsoleDisable.IsFalse() || topics[i].WebconsoleDisable.IsNone() {
			return true
		}
	}
	return false
}

func (nm *SNotificationManager) create(ctx context.Context, userCred mcclient.TokenCredential, contactType string, receiverIds, contacts []string, priority, eventId string, topicType string) error {
	if len(receiverIds)+len(contacts) == 0 {
		return nil
	}

	n := &SNotification{
		ContactType: contactType,
		Priority:    priority,
		ReceivedAt:  time.Now(),
		EventId:     eventId,
		TopicType:   topicType,
	}
	n.Id = db.DefaultUUIDGenerator()
	err := nm.TableSpec().Insert(ctx, n)
	if err != nil {
		return errors.Wrap(err, "unable to insert Notification")
	}
	for i := range receiverIds {
		_, err := ReceiverNotificationManager.Create(ctx, userCred, receiverIds[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.Create")
		}
	}
	for i := range contacts {
		_, err := ReceiverNotificationManager.CreateContact(ctx, userCred, contacts[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.CreateContact")
		}
	}
	n.SetModelManager(nm, n)
	task, err := taskman.TaskManager.NewTask(ctx, "NotificationSendTask", n, userCred, nil, "", "")
	if err != nil {
		log.Errorf("NotificationSendTask newTask error %v", err)
	} else {
		task.ScheduleRun(nil)
	}
	return nil
}

func (nm *SNotificationManager) createWithWebhookRobots(ctx context.Context, userCred mcclient.TokenCredential, webhookRobotIds []string, priority, eventId string, topicType string) error {
	if len(webhookRobotIds) == 0 {
		return nil
	}
	n := &SNotification{
		ContactType: api.WEBHOOK,
		Priority:    priority,
		ReceivedAt:  time.Now(),
		EventId:     eventId,
		TopicType:   topicType,
	}
	n.Id = db.DefaultUUIDGenerator()
	for i := range webhookRobotIds {
		_, err := ReceiverNotificationManager.CreateRobot(ctx, userCred, webhookRobotIds[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.CreateRobot")
		}
	}
	err := nm.TableSpec().Insert(ctx, n)
	if err != nil {
		return errors.Wrap(err, "unable to insert Notification")
	}
	n.SetModelManager(nm, n)
	task, err := taskman.TaskManager.NewTask(ctx, "NotificationSendTask", n, userCred, nil, "", "")
	if err != nil {
		return errors.Wrapf(err, "NewTask")
	}
	return task.ScheduleRun(nil)
}

func (nm *SNotificationManager) createWithRobots(ctx context.Context, userCred mcclient.TokenCredential, robotIds []string, priority, eventId string, topicType string) error {
	if len(robotIds) == 0 {
		return nil
	}
	n := &SNotification{
		ContactType: api.ROBOT,
		Priority:    priority,
		ReceivedAt:  time.Now(),
		EventId:     eventId,
		TopicType:   topicType,
	}
	n.Id = db.DefaultUUIDGenerator()
	for i := range robotIds {
		_, err := ReceiverNotificationManager.CreateRobot(ctx, userCred, robotIds[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.CreateRobot")
		}
	}
	err := nm.TableSpec().Insert(ctx, n)
	if err != nil {
		return errors.Wrap(err, "unable to insert Notification")
	}
	n.SetModelManager(nm, n)
	task, err := taskman.TaskManager.NewTask(ctx, "NotificationSendTask", n, userCred, nil, "", "")
	if err != nil {
		log.Errorf("NotificationSendTask newTask error %v", err)
	} else {
		task.ScheduleRun(nil)
	}
	return nil
}

func (n *SNotification) Create(ctx context.Context, userCred mcclient.TokenCredential, receiverIds, contacts []string) error {
	if len(receiverIds)+len(contacts) == 0 {
		return nil
	}

	n.Id = db.DefaultUUIDGenerator()
	err := NotificationManager.TableSpec().Insert(ctx, n)
	if err != nil {
		return errors.Wrap(err, "unable to insert Notification")
	}
	for i := range receiverIds {
		_, err := ReceiverNotificationManager.Create(ctx, userCred, receiverIds[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.Create")
		}
	}
	for i := range contacts {
		_, err := ReceiverNotificationManager.CreateContact(ctx, userCred, contacts[i], n.Id)
		if err != nil {
			return errors.Wrap(err, "ReceiverNotificationManager.CreateContact")
		}
	}
	task, err := taskman.TaskManager.NewTask(ctx, "NotificationSendTask", n, userCred, nil, "", "")
	if err != nil {
		log.Errorf("NotificationSendTask newTask error %v", err)
	} else {
		task.ScheduleRun(nil)
	}
	return nil
}

func (nm *SNotificationManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.NotificationDetails {
	rows := make([]api.NotificationDetails, len(objs))
	resRows := nm.SStatusStandaloneResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)

	var err error
	notifications := make([]*SNotification, len(objs))
	for i := range notifications {
		notifications[i] = objs[i].(*SNotification)
	}

	for i := range rows {
		rows[i], err = notifications[i].getMoreDetails(ctx, userCred, query, rows[i])
		if err != nil {
			log.Errorf("Notification.getMoreDetails: %v", err)
		}
		rows[i].StatusStandaloneResourceDetails = resRows[i]
	}
	return rows
}

func (n *SNotification) ReceiverNotificationsNotOK() ([]SReceiverNotification, error) {
	rnq := ReceiverNotificationManager.Query().Equals("notification_id", n.Id).NotEquals("status", api.RECEIVER_NOTIFICATION_OK)
	rns := make([]SReceiverNotification, 0, 1)
	err := db.FetchModelObjects(ReceiverNotificationManager, rnq, &rns)
	if err == sql.ErrNoRows {
		return []SReceiverNotification{}, nil
	}
	if err != nil {
		return nil, err
	}
	return rns, nil
}

func (n *SNotification) receiveDetails(userCred mcclient.TokenCredential, scope string) ([]api.ReceiveDetail, error) {
	RQ := ReceiverManager.Query("id", "name")
	q := ReceiverNotificationManager.Query("receiver_id", "notification_id", "receiver_type", "contact", "send_at", "send_by", "status", "failed_reason").Equals("notification_id", n.Id)
	s := rbacscope.TRbacScope(scope)

	switch s {
	case rbacscope.ScopeSystem:
		subRQ := RQ.SubQuery()
		q.AppendField(subRQ.Field("name", "receiver_name"))
		q = q.LeftJoin(subRQ, sqlchemy.OR(sqlchemy.Equals(q.Field("receiver_id"), subRQ.Field("id")), sqlchemy.Equals(q.Field("contact"), subRQ.Field("id"))))
	case rbacscope.ScopeDomain:
		subRQ := RQ.Equals("domain_id", userCred.GetDomainId()).SubQuery()
		q.AppendField(subRQ.Field("name", "receiver_name"))
		q = q.Join(subRQ, sqlchemy.OR(sqlchemy.Equals(q.Field("receiver_id"), subRQ.Field("id")), sqlchemy.Equals(q.Field("contact"), subRQ.Field("id"))))
	default:
		subRQ := RQ.Equals("id", userCred.GetUserId()).SubQuery()
		q.AppendField(subRQ.Field("name", "receiver_name"))
		q = q.Join(subRQ, sqlchemy.OR(sqlchemy.Equals(q.Field("receiver_id"), subRQ.Field("id")), sqlchemy.Equals(q.Field("contact"), subRQ.Field("id"))))
	}
	ret := make([]api.ReceiveDetail, 0, 2)
	err := q.All(&ret)
	if err != nil && errors.Cause(err) != sql.ErrNoRows {
		log.Errorf("SQuery.All: %v", err)
		return nil, err
	}
	return ret, nil
}

func (n *SNotification) getMoreDetails(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, out api.NotificationDetails) (api.NotificationDetails, error) {
	// get title adn content
	lang := getLangSuffix(ctx)
	nn, err := n.Notification()
	if err != nil {
		return out, err
	}
	// p, err := n.TemplateStore().FillWithTemplate(ctx, lang, nn)
	p, _ := n.FillWithTemplate(ctx, lang, nn)
	if err != nil {
		return out, err
	}
	out.Title = p.Title
	out.Content = p.Message

	scope, _ := query.GetString("scope")
	// get receive details
	out.ReceiveDetails, err = n.receiveDetails(userCred, scope)
	if err != nil {
		return out, err
	}
	return out, nil
}

func (n *SNotification) Notification() (api.SsNotification, error) {
	if n.EventId == "" {
		return api.SsNotification{
			ContactType: n.ContactType,
			Topic:       n.Topic,
			Message:     n.Message,
		}, nil
	}
	event, err := EventManager.GetEvent(n.EventId)
	if err != nil {
		return api.SsNotification{}, err
	}
	e, _ := parseEvent(event.Event)
	return api.SsNotification{
		ContactType: n.ContactType,
		Topic:       n.Topic,
		Message:     event.Message,
		Event:       e,
		AdvanceDays: event.AdvanceDays,
	}, nil
}

func (nm *SNotificationManager) ResourceScope() rbacscope.TRbacScope {
	return rbacscope.ScopeUser
}

func (nm *SNotificationManager) NamespaceScope() rbacscope.TRbacScope {
	return rbacscope.ScopeSystem
}

func (nm *SNotificationManager) FetchOwnerId(ctx context.Context, data jsonutils.JSONObject) (mcclient.IIdentityProvider, error) {
	return db.FetchUserInfo(ctx, data)
}

func (nm *SNotificationManager) FilterByOwner(q *sqlchemy.SQuery, owner mcclient.IIdentityProvider, scope rbacscope.TRbacScope) *sqlchemy.SQuery {
	if owner == nil {
		return q
	}
	switch scope {
	case rbacscope.ScopeDomain:
		subRq := ReceiverManager.Query("id").Equals("domain_id", owner.GetDomainId()).SubQuery()
		RNq := ReceiverNotificationManager.Query("notification_id", "receiver_id")
		subRNq := RNq.Join(subRq, sqlchemy.OR(
			sqlchemy.Equals(RNq.Field("receiver_id"), subRq.Field("id")),
			sqlchemy.Equals(RNq.Field("contact"), subRq.Field("id")),
		)).SubQuery()
		q = q.Join(subRNq, sqlchemy.Equals(q.Field("id"), subRNq.Field("notification_id")))
	case rbacscope.ScopeProject, rbacscope.ScopeUser:
		sq := ReceiverNotificationManager.Query("notification_id")
		subq := sq.Filter(sqlchemy.OR(
			sqlchemy.Equals(sq.Field("receiver_id"), owner.GetUserId()),
			sqlchemy.Equals(sq.Field("contact"), owner.GetUserId()),
		)).SubQuery()
		q = q.Join(subq, sqlchemy.Equals(q.Field("id"), subq.Field("notification_id")))
	}
	return q
}

func (n *SNotification) AddOne() error {
	_, err := db.Update(n, func() error {
		n.SendTimes += 1
		return nil
	})
	return err
}

func (self *SNotificationManager) InitializeData() error {
	return dataCleaning(self.TableSpec().Name())
}

func dataCleaning(tableName string) error {
	now := time.Now()
	monthsDaysAgo := now.AddDate(0, -1, 0).Format("2006-01-02 15:04:05")
	sqlStr := fmt.Sprintf(
		"delete from %s  where deleted = 0 and created_at < '%s'",
		tableName,
		monthsDaysAgo,
	)
	q := sqlchemy.NewRawQuery(sqlStr)
	rows, err := q.Rows()
	if err != nil {
		return errors.Wrapf(err, "unable to delete expired data in %q", tableName)
	}
	defer rows.Close()
	log.Infof("delete expired data in %q successfully", tableName)
	return nil
}

// 通知消息列表
func (nm *SNotificationManager) ListItemFilter(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, input api.NotificationListInput) (*sqlchemy.SQuery, error) {
	q, err := nm.SStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, input.StandaloneResourceListInput)
	if err != nil {
		return nil, err
	}
	if len(input.ContactType) > 0 {
		q = q.Equals("contact_type", input.ContactType)
	}
	if len(input.ReceiverId) > 0 {
		subq := ReceiverNotificationManager.Query("notification_id").Equals("receiver_id", input.ReceiverId).SubQuery()
		q = q.Join(subq, sqlchemy.Equals(q.Field("id"), subq.Field("notification_id")))
	}
	if len(input.Tag) > 0 {
		q = q.Equals("tag", input.Tag)
	}
	if len(input.TopicType) > 0 {
		q = q.Equals("topic_type", input.TopicType)
	}
	return q, nil
}

func (nm *SNotificationManager) ReSend(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	timeLimit := time.Now().Add(-time.Duration(options.Options.ReSendScope) * time.Second * 2).Format("2006-01-02 15:04:05")
	q := nm.Query().GT("created_at", timeLimit).In("status", []string{api.NOTIFICATION_STATUS_FAILED, api.NOTIFICATION_STATUS_PART_OK}).LT("send_times", options.Options.MaxSendTimes)
	ns := make([]SNotification, 0, 2)
	err := db.FetchModelObjects(nm, q, &ns)
	if err != nil {
		log.Errorf("fail to FetchModelObjects: %v", err)
		return
	}
	log.Infof("need to resend total %d notifications", len(ns))
	for i := range ns {
		task, err := taskman.TaskManager.NewTask(ctx, "NotificationSendTask", &ns[i], userCred, nil, "", "")
		if err != nil {
			log.Errorf("NotificationSendTask newTask error %v", err)
		} else {
			task.ScheduleRun(nil)
		}
	}
}

func (n *SNotification) FillWithTemplate(ctx context.Context, lang string, no api.SsNotification) (api.SendParams, error) {
	if len(n.EventId) == 0 || n.ContactType == api.MOBILE {
		return TemplateManager.FillWithTemplate(ctx, lang, no)
	}
	return LocalTemplateManager.FillWithTemplate(ctx, lang, no)
}

func (n *SNotification) GetNotOKReceivers() ([]SReceiver, error) {
	ret := []SReceiver{}
	q := ReceiverManager.Query().IsTrue("enabled")
	sq := ReceiverNotificationManager.Query().Equals("notification_id", n.Id).NotEquals("status", api.RECEIVER_NOTIFICATION_OK).Equals("receiver_type", api.RECEIVER_TYPE_USER).SubQuery()
	q = q.Join(sq, sqlchemy.Equals(q.Field("id"), sq.Field("receiver_id")))
	err := db.FetchModelObjects(ReceiverManager, q, &ret)
	return ret, err
}

func (n *SNotification) TaskInsert() error {
	return NotificationManager.TableSpec().Insert(context.Background(), n)
}
