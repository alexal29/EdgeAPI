package models

import (
	"crypto/md5"
	"fmt"
	"github.com/TeaOSLab/EdgeAPI/internal/errors"
	_ "github.com/go-sql-driver/mysql"
	"github.com/iwind/TeaGo/Tea"
	"github.com/iwind/TeaGo/dbs"
	"github.com/iwind/TeaGo/types"
	timeutil "github.com/iwind/TeaGo/utils/time"
	"time"
)

const (
	MessageStateEnabled  = 1 // 已启用
	MessageStateDisabled = 0 // 已禁用

	MessageLevelInfo    = "info"
	MessageLevelWarning = "warning"
	MessageLevelError   = "error"
	MessageLevelSuccess = "success"
)

type MessageType = string

const (
	MessageTypeHealthCheckFailed          MessageType = "HealthCheckFailed"          // 节点健康检查失败
	MessageTypeHealthCheckNodeUp          MessageType = "HealthCheckNodeUp"          // 因健康检查节点上线
	MessageTypeHealthCheckNodeDown        MessageType = "HealthCheckNodeDown"        // 因健康检查节点下线
	MessageTypeNodeInactive               MessageType = "NodeInactive"               // 节点不活跃
	MessageTypeNodeActive                 MessageType = "NodeActive"                 // 节点活跃
	MessageTypeClusterDNSSyncFailed       MessageType = "ClusterDNSSyncFailed"       // DNS同步失败
	MessageTypeSSLCertExpiring            MessageType = "SSLCertExpiring"            // SSL证书即将过期
	MessageTypeSSLCertACMETaskFailed      MessageType = "SSLCertACMETaskFailed"      // SSL证书任务执行失败
	MessageTypeSSLCertACMETaskSuccess     MessageType = "SSLCertACMETaskSuccess"     // SSL证书任务执行成功
	MessageTypeLogCapacityOverflow        MessageType = "LogCapacityOverflow"        // 日志超出最大限制
	MessageTypeServerNamesAuditingSuccess MessageType = "ServerNamesAuditingSuccess" // 服务域名审核成功
	MessageTypeServerNamesAuditingFailed  MessageType = "ServerNamesAuditingFailed"  // 服务域名审核失败
	MessageTypeThresholdSatisfied         MessageType = "ThresholdSatisfied"         // 满足阈值
)

type MessageDAO dbs.DAO

func NewMessageDAO() *MessageDAO {
	return dbs.NewDAO(&MessageDAO{
		DAOObject: dbs.DAOObject{
			DB:     Tea.Env,
			Table:  "edgeMessages",
			Model:  new(Message),
			PkName: "id",
		},
	}).(*MessageDAO)
}

var SharedMessageDAO *MessageDAO

func init() {
	dbs.OnReady(func() {
		SharedMessageDAO = NewMessageDAO()
	})
}

// EnableMessage 启用条目
func (this *MessageDAO) EnableMessage(tx *dbs.Tx, id int64) error {
	_, err := this.Query(tx).
		Pk(id).
		Set("state", MessageStateEnabled).
		Update()
	return err
}

// DisableMessage 禁用条目
func (this *MessageDAO) DisableMessage(tx *dbs.Tx, id int64) error {
	_, err := this.Query(tx).
		Pk(id).
		Set("state", MessageStateDisabled).
		Update()
	return err
}

// FindEnabledMessage 查找启用中的条目
func (this *MessageDAO) FindEnabledMessage(tx *dbs.Tx, id int64) (*Message, error) {
	result, err := this.Query(tx).
		Pk(id).
		Attr("state", MessageStateEnabled).
		Find()
	if result == nil {
		return nil, err
	}
	return result.(*Message), err
}

// CreateClusterMessage 创建集群消息
func (this *MessageDAO) CreateClusterMessage(tx *dbs.Tx, clusterId int64, messageType MessageType, level string, subject string, body string, paramsJSON []byte) error {
	_, err := this.createMessage(tx, clusterId, 0, messageType, level, subject, body, paramsJSON)
	if err != nil {
		return err
	}

	// 发送给媒介接收人
	err = SharedMessageTaskDAO.CreateMessageTasks(tx, MessageTaskTarget{
		ClusterId: clusterId,
		NodeId:    0,
		ServerId:  0,
	}, messageType, subject, body)
	if err != nil {
		return err
	}

	return nil
}

// CreateNodeMessage 创建节点消息
func (this *MessageDAO) CreateNodeMessage(tx *dbs.Tx, clusterId int64, nodeId int64, messageType MessageType, level string, subject string, body string, paramsJSON []byte) error {
	// 检查N分钟内是否已经发送过
	hash := this.calHash(subject, body, paramsJSON)
	exists, err := this.Query(tx).
		Attr("hash", hash).
		Gt("createdAt", time.Now().Unix()-10*60). // 10分钟
		Exist()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = this.createMessage(tx, clusterId, nodeId, messageType, level, subject, body, paramsJSON)
	if err != nil {
		return err
	}

	// 发送给媒介接收人 - 集群
	err = SharedMessageTaskDAO.CreateMessageTasks(tx, MessageTaskTarget{
		ClusterId: clusterId,
		NodeId:    0,
		ServerId:  0,
	}, messageType, subject, body)
	if err != nil {
		return err
	}

	// 发送给媒介接收人 - 节点
	if nodeId > 0 {
		err = SharedMessageTaskDAO.CreateMessageTasks(tx, MessageTaskTarget{
			ClusterId: clusterId,
			NodeId:    nodeId,
			ServerId:  0,
		}, messageType, subject, body)
		if err != nil {
			return err
		}
	}

	return nil
}

// CreateMessage 创建普通消息
func (this *MessageDAO) CreateMessage(tx *dbs.Tx, adminId int64, userId int64, messageType MessageType, level string, subject string, body string, paramsJSON []byte) error {
	op := NewMessageOperator()
	op.AdminId = adminId
	op.UserId = userId
	op.Type = messageType
	op.Level = level

	subjectRunes := []rune(subject)
	if len(subjectRunes) > 100 {
		op.Subject = string(subjectRunes[:100]) + "..."
	} else {
		op.Subject = subject
	}

	op.Body = body
	if len(paramsJSON) > 0 {
		op.Params = paramsJSON
	}
	op.State = MessageStateEnabled
	op.IsRead = false
	op.Day = timeutil.Format("Ymd")
	op.Hash = this.calHash(subject, body, paramsJSON)
	err := this.Save(tx, op)
	if err != nil {
		return err
	}

	return nil
}

// DeleteMessagesBeforeDay 删除某天之前的消息
func (this *MessageDAO) DeleteMessagesBeforeDay(tx *dbs.Tx, dayTime time.Time) error {
	day := timeutil.Format("Ymd", dayTime)
	_, err := this.Query(tx).
		Where("day<:day").
		Param("day", day).
		Delete()
	return err
}

// CountUnreadMessages 计算未读消息数量
func (this *MessageDAO) CountUnreadMessages(tx *dbs.Tx, adminId int64, userId int64) (int64, error) {
	query := this.Query(tx).
		Attr("isRead", false)
	if adminId > 0 {
		query.Where("(adminId=:adminId OR (adminId=0 AND userId=0))").
			Param("adminId", adminId)
	} else if userId > 0 {
		query.Attr("userId", userId)
	}
	return query.Count()
}

// ListUnreadMessages 列出单页未读消息
func (this *MessageDAO) ListUnreadMessages(tx *dbs.Tx, adminId int64, userId int64, offset int64, size int64) (result []*Message, err error) {
	query := this.Query(tx).
		Attr("isRead", false)
	if adminId > 0 {
		query.Where("(adminId=:adminId OR (adminId=0 AND userId=0))").
			Param("adminId", adminId)
	} else if userId > 0 {
		query.Attr("userId", userId)
	}
	_, err = query.
		Offset(offset).
		Limit(size).
		DescPk().
		Slice(&result).
		FindAll()
	return
}

// UpdateMessageRead 设置消息已读状态
func (this *MessageDAO) UpdateMessageRead(tx *dbs.Tx, messageId int64, b bool) error {
	if messageId <= 0 {
		return errors.New("invalid messageId")
	}
	op := NewMessageOperator()
	op.Id = messageId
	op.IsRead = b
	err := this.Save(tx, op)
	return err
}

// UpdateMessagesRead 设置一组消息为已读状态
func (this *MessageDAO) UpdateMessagesRead(tx *dbs.Tx, messageIds []int64, b bool) error {
	// 这里我们一个一个更改，因为In语句不容易Prepare，且效率不高
	for _, messageId := range messageIds {
		err := this.UpdateMessageRead(tx, messageId, b)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateAllMessagesRead 设置所有消息为已读
func (this *MessageDAO) UpdateAllMessagesRead(tx *dbs.Tx, adminId int64, userId int64) error {
	query := this.Query(tx).
		Attr("isRead", false)
	if adminId > 0 {
		query.Where("(adminId=:adminId OR (adminId=0 AND userId=0))").
			Param("adminId", adminId)
	} else if userId > 0 {
		query.Attr("userId", userId)
	}
	_, err := query.
		Set("isRead", true).
		Update()
	return err
}

// CheckMessageUser 检查消息权限
func (this *MessageDAO) CheckMessageUser(tx *dbs.Tx, messageId int64, adminId int64, userId int64) (bool, error) {
	if messageId <= 0 || (adminId <= 0 && userId <= 0) {
		return false, nil
	}
	query := this.Query(tx).
		Pk(messageId)
	if adminId > 0 {
		query.Where("(adminId=:adminId OR (adminId=0 AND userId=0))").
			Param("adminId", adminId)
	} else if userId > 0 {
		query.Attr("userId", userId)
	}
	return query.Exist()
}

// 创建消息
func (this *MessageDAO) createMessage(tx *dbs.Tx, clusterId int64, nodeId int64, messageType MessageType, level string, subject string, body string, paramsJSON []byte) (int64, error) {
	// TODO 检查同样的消息最近是否发送过

	// 创建新消息
	op := NewMessageOperator()
	op.AdminId = 0 // TODO
	op.UserId = 0  // TODO
	op.ClusterId = clusterId
	op.NodeId = nodeId
	op.Type = messageType
	op.Level = level

	subjectRunes := []rune(subject)
	if len(subjectRunes) > 100 {
		op.Subject = string(subjectRunes[:100]) + "..."
	} else {
		op.Subject = subject
	}

	op.Body = body
	if len(paramsJSON) > 0 {
		op.Params = paramsJSON
	}
	op.IsRead = false
	op.State = MessageStateEnabled
	op.CreatedAt = time.Now().Unix()
	op.Day = timeutil.Format("Ymd")
	op.Hash = this.calHash(subject, body, paramsJSON)

	err := this.Save(tx, op)
	if err != nil {
		return 0, err
	}
	return types.Int64(op.Id), nil
}

// 计算Hash
func (this *MessageDAO) calHash(subject string, body string, paramsJSON []byte) string {
	h := md5.New()
	h.Write([]byte(subject))
	h.Write([]byte(body))
	h.Write(paramsJSON)
	return fmt.Sprintf("%x", h.Sum(nil))
}
