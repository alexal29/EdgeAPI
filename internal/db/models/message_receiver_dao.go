package models

import (
	"encoding/json"
	_ "github.com/go-sql-driver/mysql"
	"github.com/iwind/TeaGo/Tea"
	"github.com/iwind/TeaGo/dbs"
	"github.com/iwind/TeaGo/maps"
)

const (
	MessageReceiverStateEnabled  = 1 // 已启用
	MessageReceiverStateDisabled = 0 // 已禁用
)

type MessageReceiverDAO dbs.DAO

func NewMessageReceiverDAO() *MessageReceiverDAO {
	return dbs.NewDAO(&MessageReceiverDAO{
		DAOObject: dbs.DAOObject{
			DB:     Tea.Env,
			Table:  "edgeMessageReceivers",
			Model:  new(MessageReceiver),
			PkName: "id",
		},
	}).(*MessageReceiverDAO)
}

var SharedMessageReceiverDAO *MessageReceiverDAO

func init() {
	dbs.OnReady(func() {
		SharedMessageReceiverDAO = NewMessageReceiverDAO()
	})
}

// EnableMessageReceiver 启用条目
func (this *MessageReceiverDAO) EnableMessageReceiver(tx *dbs.Tx, id int64) error {
	_, err := this.Query(tx).
		Pk(id).
		Set("state", MessageReceiverStateEnabled).
		Update()
	return err
}

// DisableMessageReceiver 禁用条目
func (this *MessageReceiverDAO) DisableMessageReceiver(tx *dbs.Tx, id int64) error {
	_, err := this.Query(tx).
		Pk(id).
		Set("state", MessageReceiverStateDisabled).
		Update()
	return err
}

// FindEnabledMessageReceiver 查找启用中的条目
func (this *MessageReceiverDAO) FindEnabledMessageReceiver(tx *dbs.Tx, id int64) (*MessageReceiver, error) {
	result, err := this.Query(tx).
		Pk(id).
		Attr("state", MessageReceiverStateEnabled).
		Find()
	if result == nil {
		return nil, err
	}
	return result.(*MessageReceiver), err
}

// DisableReceivers 禁用一组接收人
func (this *MessageReceiverDAO) DisableReceivers(tx *dbs.Tx, clusterId int64, nodeId int64, serverId int64) error {
	return this.Query(tx).
		Attr("clusterId", clusterId).
		Attr("nodeId", nodeId).
		Attr("serverId", serverId).
		Set("state", MessageReceiverStateDisabled).
		UpdateQuickly()
}

// CreateReceiver 创建接收人
func (this *MessageReceiverDAO) CreateReceiver(tx *dbs.Tx, target MessageTaskTarget, messageType MessageType, params maps.Map, recipientId int64, recipientGroupId int64) (int64, error) {
	op := NewMessageReceiverOperator()
	op.ClusterId = target.ClusterId
	op.NodeId = target.NodeId
	op.ServerId = target.ServerId
	op.Type = messageType

	if params == nil {
		params = maps.Map{}
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}
	op.Params = paramsJSON

	op.RecipientId = recipientId
	op.RecipientGroupId = recipientGroupId
	op.State = MessageReceiverStateEnabled
	return this.SaveInt64(tx, op)
}

// FindAllEnabledReceivers 查询接收人
func (this *MessageReceiverDAO) FindAllEnabledReceivers(tx *dbs.Tx, target MessageTaskTarget, messageType string) (result []*MessageReceiver, err error) {
	query := this.Query(tx)
	if len(messageType) > 0 {
		query.Attr("type", []string{"*", messageType}) // *表示所有的
	}
	_, err = query.
		Attr("clusterId", target.ClusterId).
		Attr("nodeId", target.NodeId).
		Attr("serverId", target.ServerId).
		State(MessageReceiverStateEnabled).
		AscPk().
		Slice(&result).
		FindAll()
	return
}

// CountAllEnabledReceivers 计算接收人数量
func (this *MessageReceiverDAO) CountAllEnabledReceivers(tx *dbs.Tx, target MessageTaskTarget, messageType string) (int64, error) {
	query := this.Query(tx)
	if len(messageType) > 0 {
		query.Attr("type", []string{"*", messageType}) // *表示所有的
	}
	return query.
		Attr("clusterId", target.ClusterId).
		Attr("nodeId", target.NodeId).
		Attr("serverId", target.ServerId).
		State(MessageReceiverStateEnabled).
		Count()
}
