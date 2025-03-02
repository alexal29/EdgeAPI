package models

//
type MessageTask struct {
	Id          uint64 `field:"id"`          // ID
	RecipientId uint32 `field:"recipientId"` // 接收人ID
	InstanceId  uint32 `field:"instanceId"`  // 媒介实例ID
	User        string `field:"user"`        // 接收用户标识
	Subject     string `field:"subject"`     // 标题
	Body        string `field:"body"`        // 内容
	CreatedAt   uint64 `field:"createdAt"`   // 创建时间
	Status      uint8  `field:"status"`      // 发送状态
	SentAt      uint64 `field:"sentAt"`      // 最后一次发送时间
	State       uint8  `field:"state"`       // 状态
	Result      string `field:"result"`      // 结果
	IsPrimary   uint8  `field:"isPrimary"`   // 是否优先
}

type MessageTaskOperator struct {
	Id          interface{} // ID
	RecipientId interface{} // 接收人ID
	InstanceId  interface{} // 媒介实例ID
	User        interface{} // 接收用户标识
	Subject     interface{} // 标题
	Body        interface{} // 内容
	CreatedAt   interface{} // 创建时间
	Status      interface{} // 发送状态
	SentAt      interface{} // 最后一次发送时间
	State       interface{} // 状态
	Result      interface{} // 结果
	IsPrimary   interface{} // 是否优先
}

func NewMessageTaskOperator() *MessageTaskOperator {
	return &MessageTaskOperator{}
}
