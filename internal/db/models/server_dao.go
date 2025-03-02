package models

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/TeaOSLab/EdgeAPI/internal/db/models/dns"
	"github.com/TeaOSLab/EdgeAPI/internal/utils/numberutils"
	"github.com/TeaOSLab/EdgeCommon/pkg/configutils"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
	"github.com/TeaOSLab/EdgeCommon/pkg/serverconfigs"
	"github.com/TeaOSLab/EdgeCommon/pkg/systemconfigs"
	_ "github.com/go-sql-driver/mysql"
	"github.com/iwind/TeaGo/Tea"
	"github.com/iwind/TeaGo/dbs"
	"github.com/iwind/TeaGo/lists"
	"github.com/iwind/TeaGo/maps"
	"github.com/iwind/TeaGo/rands"
	"github.com/iwind/TeaGo/types"
	"strconv"
	"strings"
	"time"
)

const (
	ServerStateEnabled  = 1 // 已启用
	ServerStateDisabled = 0 // 已禁用
)

type ServerDAO dbs.DAO

func NewServerDAO() *ServerDAO {
	return dbs.NewDAO(&ServerDAO{
		DAOObject: dbs.DAOObject{
			DB:     Tea.Env,
			Table:  "edgeServers",
			Model:  new(Server),
			PkName: "id",
		},
	}).(*ServerDAO)
}

var SharedServerDAO *ServerDAO

func init() {
	dbs.OnReady(func() {
		SharedServerDAO = NewServerDAO()
	})
}

// Init 初始化
func (this *ServerDAO) Init() {
	this.DAOObject.Init()

	// 这里不处理增删改事件，是为了避免Server修改本身的时候，也要触发别的Server变更
}

// EnableServer 启用条目
func (this *ServerDAO) EnableServer(tx *dbs.Tx, id uint32) (rowsAffected int64, err error) {
	return this.Query(tx).
		Pk(id).
		Set("state", ServerStateEnabled).
		Update()
}

// DisableServer 禁用条目
func (this *ServerDAO) DisableServer(tx *dbs.Tx, serverId int64) (err error) {
	_, err = this.Query(tx).
		Pk(serverId).
		Set("state", ServerStateDisabled).
		Update()
	if err != nil {
		return err
	}
	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return err
	}

	err = this.NotifyDNSUpdate(tx, serverId)
	if err != nil {
		return err
	}
	return nil
}

// FindEnabledServer 查找启用中的服务
func (this *ServerDAO) FindEnabledServer(tx *dbs.Tx, serverId int64) (*Server, error) {
	result, err := this.Query(tx).
		Pk(serverId).
		Attr("state", ServerStateEnabled).
		Find()
	if result == nil {
		return nil, err
	}
	return result.(*Server), err
}

// FindEnabledServerName 查找服务名称
func (this *ServerDAO) FindEnabledServerName(tx *dbs.Tx, serverId int64) (string, error) {
	return this.Query(tx).
		Pk(serverId).
		State(ServerStateEnabled).
		Result("name").
		FindStringCol("")
}

// FindEnabledServerBasic 查找服务基本信息
func (this *ServerDAO) FindEnabledServerBasic(tx *dbs.Tx, serverId int64) (*Server, error) {
	result, err := this.Query(tx).
		Pk(serverId).
		State(ServerStateEnabled).
		Result("id", "name", "description", "isOn", "type", "clusterId").
		Find()
	if result == nil {
		return nil, err
	}
	return result.(*Server), err
}

// FindEnabledServerType 查找服务类型
func (this *ServerDAO) FindEnabledServerType(tx *dbs.Tx, serverId int64) (string, error) {
	return this.Query(tx).
		Pk(serverId).
		Result("type").
		FindStringCol("")
}

// CreateServer 创建服务
func (this *ServerDAO) CreateServer(tx *dbs.Tx,
	adminId int64,
	userId int64,
	serverType serverconfigs.ServerType,
	name string,
	description string,
	serverNamesJSON []byte,
	isAuditing bool,
	auditingServerNamesJSON []byte,
	httpJSON string,
	httpsJSON string,
	tcpJSON string,
	tlsJSON string,
	unixJSON string,
	udpJSON string,
	webId int64,
	reverseProxyJSON []byte,
	clusterId int64,
	includeNodesJSON string,
	excludeNodesJSON string,
	groupIds []int64) (serverId int64, err error) {
	op := NewServerOperator()
	op.UserId = userId
	op.AdminId = adminId
	op.Name = name
	op.Type = serverType
	op.Description = description

	if len(serverNamesJSON) > 0 {
		op.ServerNames = serverNamesJSON
	}
	op.IsAuditing = isAuditing
	if len(auditingServerNamesJSON) > 0 {
		op.AuditingServerNames = auditingServerNamesJSON
	}
	if IsNotNull(httpJSON) {
		op.Http = httpJSON
	}
	if IsNotNull(httpsJSON) {
		op.Https = httpsJSON
	}
	if IsNotNull(tcpJSON) {
		op.Tcp = tcpJSON
	}
	if IsNotNull(tlsJSON) {
		op.Tls = tlsJSON
	}
	if IsNotNull(unixJSON) {
		op.Unix = unixJSON
	}
	if IsNotNull(udpJSON) {
		op.Udp = udpJSON
	}
	op.WebId = webId
	if len(reverseProxyJSON) > 0 {
		op.ReverseProxy = reverseProxyJSON
	}

	op.ClusterId = clusterId
	if len(includeNodesJSON) > 0 {
		op.IncludeNodes = includeNodesJSON
	}
	if len(excludeNodesJSON) > 0 {
		op.ExcludeNodes = excludeNodesJSON
	}

	if len(groupIds) == 0 {
		op.GroupIds = "[]"
	} else {
		groupIdsJSON, err := json.Marshal(groupIds)
		if err != nil {
			return 0, err
		}
		op.GroupIds = groupIdsJSON
	}

	dnsName, err := this.GenDNSName(tx)
	if err != nil {
		return 0, err
	}
	op.DnsName = dnsName

	op.Version = 1
	op.IsOn = 1
	op.State = ServerStateEnabled
	err = this.Save(tx, op)

	if err != nil {
		return 0, err
	}

	serverId = types.Int64(op.Id)

	// 通知配置更改
	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return 0, err
	}

	// 通知DNS更改
	err = this.NotifyDNSUpdate(tx, serverId)
	if err != nil {
		return 0, err
	}

	return serverId, nil
}

// UpdateServerBasic 修改服务基本信息
func (this *ServerDAO) UpdateServerBasic(tx *dbs.Tx, serverId int64, name string, description string, clusterId int64, isOn bool, groupIds []int64) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	op := NewServerOperator()
	op.Id = serverId
	op.Name = name
	op.Description = description
	op.ClusterId = clusterId
	op.IsOn = isOn

	if len(groupIds) == 0 {
		op.GroupIds = "[]"
	} else {
		groupIdsJSON, err := json.Marshal(groupIds)
		if err != nil {
			return err
		}
		op.GroupIds = groupIdsJSON
	}

	err := this.Save(tx, op)
	if err != nil {
		return err
	}

	// 通知更新
	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return err
	}

	// 因为可能有isOn的原因，所以需要修改
	return this.NotifyDNSUpdate(tx, serverId)
}

// UpdateUserServerBasic 设置用户相关的基本信息
func (this *ServerDAO) UpdateUserServerBasic(tx *dbs.Tx, serverId int64, name string) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	op := NewServerOperator()
	op.Id = serverId
	op.Name = name

	err := this.Save(tx, op)
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerIsOn 修复服务是否启用
func (this *ServerDAO) UpdateServerIsOn(tx *dbs.Tx, serverId int64, isOn bool) error {
	_, err := this.Query(tx).
		Pk(serverId).
		Set("isOn", isOn).
		Update()
	if err != nil {
		return err
	}

	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return err
	}

	return nil
}

// UpdateServerConfig 修改服务配置
func (this *ServerDAO) UpdateServerConfig(tx *dbs.Tx, serverId int64, configJSON []byte, updateMd5 bool) (isChanged bool, err error) {
	if serverId <= 0 {
		return false, errors.New("serverId should not be smaller than 0")
	}

	// 查询以前的md5
	oldConfigMd5, err := this.Query(tx).
		Pk(serverId).
		Result("configMd5").
		FindStringCol("")
	if err != nil {
		return false, err
	}

	globalConfig, err := SharedSysSettingDAO.ReadSetting(tx, systemconfigs.SettingCodeServerGlobalConfig)
	if err != nil {
		return false, err
	}

	m := md5.New()
	_, _ = m.Write(configJSON)   // 当前服务配置
	_, _ = m.Write(globalConfig) // 全局配置
	h := m.Sum(nil)
	newConfigMd5 := fmt.Sprintf("%x", h)

	// 如果配置相同则不更改
	if oldConfigMd5 == newConfigMd5 {
		return false, nil
	}

	op := NewServerOperator()
	op.Id = serverId
	op.Config = JSONBytes(configJSON)
	op.Version = dbs.SQL("version+1")

	if updateMd5 {
		op.ConfigMd5 = newConfigMd5
	}
	err = this.Save(tx, op)
	return true, err
}

// UpdateServerHTTP 修改HTTP配置
func (this *ServerDAO) UpdateServerHTTP(tx *dbs.Tx, serverId int64, config []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	if len(config) == 0 {
		config = []byte("null")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("http", string(config)).
		Update()
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerHTTPS 修改HTTPS配置
func (this *ServerDAO) UpdateServerHTTPS(tx *dbs.Tx, serverId int64, httpsJSON []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	if len(httpsJSON) == 0 {
		httpsJSON = []byte("null")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("https", string(httpsJSON)).
		Update()
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerTCP 修改TCP配置
func (this *ServerDAO) UpdateServerTCP(tx *dbs.Tx, serverId int64, config []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	if len(config) == 0 {
		config = []byte("null")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("tcp", string(config)).
		Update()
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerTLS 修改TLS配置
func (this *ServerDAO) UpdateServerTLS(tx *dbs.Tx, serverId int64, config []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	if len(config) == 0 {
		config = []byte("null")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("tls", string(config)).
		Update()
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerUnix 修改Unix配置
func (this *ServerDAO) UpdateServerUnix(tx *dbs.Tx, serverId int64, config []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	if len(config) == 0 {
		config = []byte("null")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("unix", string(config)).
		Update()
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerUDP 修改UDP配置
func (this *ServerDAO) UpdateServerUDP(tx *dbs.Tx, serverId int64, config []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	if len(config) == 0 {
		config = []byte("null")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("udp", string(config)).
		Update()
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerWeb 修改Web配置
func (this *ServerDAO) UpdateServerWeb(tx *dbs.Tx, serverId int64, webId int64) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	_, err := this.Query(tx).
		Pk(serverId).
		Set("webId", webId).
		Update()
	if err != nil {
		return err
	}
	return this.NotifyUpdate(tx, serverId)
}

// InitServerWeb 初始化Web配置
func (this *ServerDAO) InitServerWeb(tx *dbs.Tx, serverId int64) (int64, error) {
	if serverId <= 0 {
		return 0, errors.New("serverId should not be smaller than 0")
	}

	adminId, userId, err := this.FindServerAdminIdAndUserId(tx, serverId)
	if err != nil {
		return 0, err
	}

	webId, err := SharedHTTPWebDAO.CreateWeb(tx, adminId, userId, nil)
	if err != nil {
		return 0, err
	}

	_, err = this.Query(tx).
		Pk(serverId).
		Set("webId", webId).
		Update()
	if err != nil {
		return 0, err
	}

	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return webId, err
	}

	return webId, nil
}

// FindServerServerNames 查找ServerNames配置
func (this *ServerDAO) FindServerServerNames(tx *dbs.Tx, serverId int64) (serverNamesJSON []byte, isAuditing bool, auditingServerNamesJSON []byte, auditingResultJSON []byte, err error) {
	if serverId <= 0 {
		return
	}
	one, err := this.Query(tx).
		Pk(serverId).
		Result("serverNames", "isAuditing", "auditingServerNames", "auditingResult").
		Find()
	if err != nil {
		return nil, false, nil, nil, err
	}
	if one == nil {
		return
	}
	server := one.(*Server)
	return []byte(server.ServerNames), server.IsAuditing == 1, []byte(server.AuditingServerNames), []byte(server.AuditingResult), nil
}

// UpdateServerNames 修改ServerNames配置
func (this *ServerDAO) UpdateServerNames(tx *dbs.Tx, serverId int64, serverNames []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}

	op := NewServerOperator()
	op.Id = serverId

	if len(serverNames) == 0 {
		serverNames = []byte("[]")
	}
	op.ServerNames = serverNames
	err := this.Save(tx, op)
	if err != nil {
		return err
	}
	return this.NotifyUpdate(tx, serverId)
}

// UpdateAuditingServerNames 修改域名审核
func (this *ServerDAO) UpdateAuditingServerNames(tx *dbs.Tx, serverId int64, isAuditing bool, auditingServerNamesJSON []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}

	op := NewServerOperator()
	op.Id = serverId
	op.IsAuditing = isAuditing
	if len(auditingServerNamesJSON) == 0 {
		op.AuditingServerNames = "[]"
	} else {
		op.AuditingServerNames = auditingServerNamesJSON
	}
	op.AuditingResult = `{"isOk":true}`
	err := this.Save(tx, op)
	if err != nil {
		return err
	}
	return this.NotifyUpdate(tx, serverId)
}

// UpdateServerAuditing 修改域名审核结果
func (this *ServerDAO) UpdateServerAuditing(tx *dbs.Tx, serverId int64, result *pb.ServerNameAuditingResult) error {
	if serverId <= 0 {
		return errors.New("invalid serverId")
	}

	resultJSON, err := json.Marshal(maps.Map{
		"isOk":      result.IsOk,
		"reason":    result.Reason,
		"createdAt": time.Now().Unix(),
	})
	if err != nil {
		return err
	}

	op := NewServerOperator()
	op.Id = serverId
	op.IsAuditing = false
	op.AuditingResult = resultJSON
	if result.IsOk {
		op.ServerNames = dbs.SQL("auditingServerNames")
	}
	err = this.Save(tx, op)
	if err != nil {
		return err
	}

	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return err
	}

	return this.NotifyDNSUpdate(tx, serverId)
}

// UpdateServerReverseProxy 修改反向代理配置
func (this *ServerDAO) UpdateServerReverseProxy(tx *dbs.Tx, serverId int64, config []byte) error {
	if serverId <= 0 {
		return errors.New("serverId should not be smaller than 0")
	}
	op := NewServerOperator()
	op.Id = serverId
	op.ReverseProxy = JSONBytes(config)
	err := this.Save(tx, op)
	if err != nil {
		return err
	}

	return this.NotifyUpdate(tx, serverId)
}

// CountAllEnabledServers 计算所有可用服务数量
func (this *ServerDAO) CountAllEnabledServers(tx *dbs.Tx) (int64, error) {
	return this.Query(tx).
		State(ServerStateEnabled).
		Count()
}

// CountAllEnabledServersMatch 计算所有可用服务数量
func (this *ServerDAO) CountAllEnabledServersMatch(tx *dbs.Tx, groupId int64, keyword string, userId int64, clusterId int64, auditingFlag configutils.BoolState, protocolFamily string) (int64, error) {
	query := this.Query(tx).
		State(ServerStateEnabled)
	if groupId > 0 {
		query.Where("JSON_CONTAINS(groupIds, :groupId)").
			Param("groupId", numberutils.FormatInt64(groupId))
	}
	if len(keyword) > 0 {
		query.Where("(name LIKE :keyword OR serverNames LIKE :keyword)").
			Param("keyword", "%"+keyword+"%")
	}
	if userId > 0 {
		query.Attr("userId", userId)
	}
	if clusterId > 0 {
		query.Attr("clusterId", clusterId)
	}
	if auditingFlag == configutils.BoolStateYes {
		query.Attr("isAuditing", true)
	}
	if protocolFamily == "http" {
		query.Where("(http IS NOT NULL OR https IS NOT NULL)")
	} else if protocolFamily == "tcp" {
		query.Where("(tcp IS NOT NULL OR tls IS NOT NULL)")
	}
	return query.Count()
}

// ListEnabledServersMatch 列出单页的服务
func (this *ServerDAO) ListEnabledServersMatch(tx *dbs.Tx, offset int64, size int64, groupId int64, keyword string, userId int64, clusterId int64, auditingFlag int32, protocolFamily string) (result []*Server, err error) {
	query := this.Query(tx).
		State(ServerStateEnabled).
		Offset(offset).
		Limit(size).
		DescPk().
		Slice(&result)

	if groupId > 0 {
		query.Where("JSON_CONTAINS(groupIds, :groupId)").
			Param("groupId", numberutils.FormatInt64(groupId))
	}
	if len(keyword) > 0 {
		query.Where("(name LIKE :keyword OR serverNames LIKE :keyword)").
			Param("keyword", "%"+keyword+"%")
	}
	if userId > 0 {
		query.Attr("userId", userId)
	}
	if clusterId > 0 {
		query.Attr("clusterId", clusterId)
	}
	if auditingFlag == 1 {
		query.Attr("isAuditing", true)
	}
	if protocolFamily == "http" {
		query.Where("(http IS NOT NULL OR https IS NOT NULL)")
	} else if protocolFamily == "tcp" {
		query.Where("(tcp IS NOT NULL OR tls IS NOT NULL)")
	}

	_, err = query.FindAll()
	return
}

// FindAllEnabledServersWithNode 获取节点中的所有服务
func (this *ServerDAO) FindAllEnabledServersWithNode(tx *dbs.Tx, nodeId int64) (result []*Server, err error) {
	// 节点所在集群
	clusterId, err := SharedNodeDAO.FindNodeClusterId(tx, nodeId)
	if err != nil {
		return nil, err
	}
	if clusterId <= 0 {
		return nil, nil
	}

	_, err = this.Query(tx).
		Attr("clusterId", clusterId).
		State(ServerStateEnabled).
		AscPk().
		Slice(&result).
		FindAll()
	return
}

// FindAllEnabledServerIds 获取所有的服务ID
func (this *ServerDAO) FindAllEnabledServerIds(tx *dbs.Tx) (serverIds []int64, err error) {
	ones, err := this.Query(tx).
		State(ServerStateEnabled).
		AscPk().
		ResultPk().
		FindAll()
	for _, one := range ones {
		serverIds = append(serverIds, int64(one.(*Server).Id))
	}
	return
}

// FindAllEnabledServerIdsWithUserId 获取某个用户的所有的服务ID
func (this *ServerDAO) FindAllEnabledServerIdsWithUserId(tx *dbs.Tx, userId int64) (serverIds []int64, err error) {
	ones, err := this.Query(tx).
		State(ServerStateEnabled).
		Attr("userId", userId).
		AscPk().
		ResultPk().
		FindAll()
	for _, one := range ones {
		serverIds = append(serverIds, int64(one.(*Server).Id))
	}
	return
}

// FindServerNodeFilters 查找服务的搜索条件
func (this *ServerDAO) FindServerNodeFilters(tx *dbs.Tx, serverId int64) (isOk bool, clusterId int64, err error) {
	one, err := this.Query(tx).
		Pk(serverId).
		Result("clusterId").
		Find()
	if err != nil {
		return false, 0, err
	}
	if one == nil {
		isOk = false
		return
	}
	server := one.(*Server)
	return true, int64(server.ClusterId), nil
}

// ComposeServerConfig 构造服务的Config
func (this *ServerDAO) ComposeServerConfig(tx *dbs.Tx, serverId int64) (*serverconfigs.ServerConfig, error) {
	server, err := this.FindEnabledServer(tx, serverId)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, ErrNotFound
	}

	config := &serverconfigs.ServerConfig{}
	config.Id = serverId
	config.Type = server.Type
	config.IsOn = server.IsOn == 1
	config.Name = server.Name
	config.Description = server.Description

	// ServerNames
	if len(server.ServerNames) > 0 && server.ServerNames != "null" {
		serverNames := []*serverconfigs.ServerNameConfig{}
		err = json.Unmarshal([]byte(server.ServerNames), &serverNames)
		if err != nil {
			return nil, err
		}
		config.ServerNames = serverNames
	}

	// CNAME
	if server.ClusterId > 0 && len(server.DnsName) > 0 {
		clusterDNS, err := SharedNodeClusterDAO.FindClusterDNSInfo(tx, int64(server.ClusterId))
		if err != nil {
			return nil, err
		}
		if clusterDNS != nil && clusterDNS.DnsDomainId > 0 {
			domain, err := dns.SharedDNSDomainDAO.FindEnabledDNSDomain(tx, int64(clusterDNS.DnsDomainId))
			if err != nil {
				return nil, err
			}
			if domain != nil {
				cname := server.DnsName + "." + domain.Name
				config.AliasServerNames = append(config.AliasServerNames, cname)
			}
		}
	}

	// HTTP
	if len(server.Http) > 0 && server.Http != "null" {
		httpConfig := &serverconfigs.HTTPProtocolConfig{}
		err = json.Unmarshal([]byte(server.Http), httpConfig)
		if err != nil {
			return nil, err
		}
		config.HTTP = httpConfig
	}

	// HTTPS
	if len(server.Https) > 0 && server.Https != "null" {
		httpsConfig := &serverconfigs.HTTPSProtocolConfig{}
		err = json.Unmarshal([]byte(server.Https), httpsConfig)
		if err != nil {
			return nil, err
		}

		// SSL
		if httpsConfig.SSLPolicyRef != nil && httpsConfig.SSLPolicyRef.SSLPolicyId > 0 {
			sslPolicyConfig, err := SharedSSLPolicyDAO.ComposePolicyConfig(tx, httpsConfig.SSLPolicyRef.SSLPolicyId)
			if err != nil {
				return nil, err
			}
			if sslPolicyConfig != nil {
				httpsConfig.SSLPolicy = sslPolicyConfig
			}
		}

		config.HTTPS = httpsConfig
	}

	// TCP
	if len(server.Tcp) > 0 && server.Tcp != "null" {
		tcpConfig := &serverconfigs.TCPProtocolConfig{}
		err = json.Unmarshal([]byte(server.Tcp), tcpConfig)
		if err != nil {
			return nil, err
		}
		config.TCP = tcpConfig
	}

	// TLS
	if len(server.Tls) > 0 && server.Tls != "null" {
		tlsConfig := &serverconfigs.TLSProtocolConfig{}
		err = json.Unmarshal([]byte(server.Tls), tlsConfig)
		if err != nil {
			return nil, err
		}

		// SSL
		if tlsConfig.SSLPolicyRef != nil {
			sslPolicyConfig, err := SharedSSLPolicyDAO.ComposePolicyConfig(tx, tlsConfig.SSLPolicyRef.SSLPolicyId)
			if err != nil {
				return nil, err
			}
			if sslPolicyConfig != nil {
				tlsConfig.SSLPolicy = sslPolicyConfig
			}
		}

		config.TLS = tlsConfig
	}

	// Unix
	if len(server.Unix) > 0 && server.Unix != "null" {
		unixConfig := &serverconfigs.UnixProtocolConfig{}
		err = json.Unmarshal([]byte(server.Unix), unixConfig)
		if err != nil {
			return nil, err
		}
		config.Unix = unixConfig
	}

	// UDP
	if len(server.Udp) > 0 && server.Udp != "null" {
		udpConfig := &serverconfigs.UDPProtocolConfig{}
		err = json.Unmarshal([]byte(server.Udp), udpConfig)
		if err != nil {
			return nil, err
		}
		config.UDP = udpConfig
	}

	// Web
	if server.WebId > 0 {
		webConfig, err := SharedHTTPWebDAO.ComposeWebConfig(tx, int64(server.WebId))
		if err != nil {
			return nil, err
		}
		if webConfig != nil {
			config.Web = webConfig
		}
	}

	// ReverseProxy
	if IsNotNull(server.ReverseProxy) {
		reverseProxyRef := &serverconfigs.ReverseProxyRef{}
		err = json.Unmarshal([]byte(server.ReverseProxy), reverseProxyRef)
		if err != nil {
			return nil, err
		}
		config.ReverseProxyRef = reverseProxyRef

		reverseProxyConfig, err := SharedReverseProxyDAO.ComposeReverseProxyConfig(tx, reverseProxyRef.ReverseProxyId)
		if err != nil {
			return nil, err
		}
		if reverseProxyConfig != nil {
			config.ReverseProxy = reverseProxyConfig
		}
	}

	return config, nil
}

// RenewServerConfig 更新服务的Config配置
func (this *ServerDAO) RenewServerConfig(tx *dbs.Tx, serverId int64, updateMd5 bool) (isChanged bool, err error) {
	serverConfig, err := this.ComposeServerConfig(tx, serverId)
	if err != nil {
		return false, err
	}
	data, err := json.Marshal(serverConfig)
	if err != nil {
		return false, err
	}
	return this.UpdateServerConfig(tx, serverId, data, updateMd5)
}

// FindReverseProxyRef 根据条件获取反向代理配置
func (this *ServerDAO) FindReverseProxyRef(tx *dbs.Tx, serverId int64) (*serverconfigs.ReverseProxyRef, error) {
	reverseProxy, err := this.Query(tx).
		Pk(serverId).
		Result("reverseProxy").
		FindStringCol("")
	if err != nil {
		return nil, err
	}
	if len(reverseProxy) == 0 || reverseProxy == "null" {
		return nil, nil
	}
	config := &serverconfigs.ReverseProxyRef{}
	err = json.Unmarshal([]byte(reverseProxy), config)
	return config, err
}

// FindServerWebId 查找Server对应的WebId
func (this *ServerDAO) FindServerWebId(tx *dbs.Tx, serverId int64) (int64, error) {
	webId, err := this.Query(tx).
		Pk(serverId).
		Result("webId").
		FindIntCol(0)
	if err != nil {
		return 0, err
	}
	return int64(webId), nil
}

// CountAllEnabledServersWithSSLPolicyIds 计算使用SSL策略的所有服务数量
func (this *ServerDAO) CountAllEnabledServersWithSSLPolicyIds(tx *dbs.Tx, sslPolicyIds []int64) (count int64, err error) {
	if len(sslPolicyIds) == 0 {
		return
	}
	policyStringIds := []string{}
	for _, policyId := range sslPolicyIds {
		policyStringIds = append(policyStringIds, strconv.FormatInt(policyId, 10))
	}
	return this.Query(tx).
		State(ServerStateEnabled).
		Where("(FIND_IN_SET(JSON_EXTRACT(https, '$.sslPolicyRef.sslPolicyId'), :policyIds) OR FIND_IN_SET(JSON_EXTRACT(tls, '$.sslPolicyRef.sslPolicyId'), :policyIds))").
		Param("policyIds", strings.Join(policyStringIds, ",")).
		Count()
}

// FindAllEnabledServersWithSSLPolicyIds 查找使用某个SSL策略的所有服务
func (this *ServerDAO) FindAllEnabledServersWithSSLPolicyIds(tx *dbs.Tx, sslPolicyIds []int64) (result []*Server, err error) {
	if len(sslPolicyIds) == 0 {
		return
	}
	policyStringIds := []string{}
	for _, policyId := range sslPolicyIds {
		policyStringIds = append(policyStringIds, strconv.FormatInt(policyId, 10))
	}
	_, err = this.Query(tx).
		State(ServerStateEnabled).
		Result("id", "name", "https", "tls", "isOn", "type").
		Where("(FIND_IN_SET(JSON_EXTRACT(https, '$.sslPolicyRef.sslPolicyId'), :policyIds) OR FIND_IN_SET(JSON_EXTRACT(tls, '$.sslPolicyRef.sslPolicyId'), :policyIds))").
		Param("policyIds", strings.Join(policyStringIds, ",")).
		Slice(&result).
		AscPk().
		FindAll()
	return
}

// FindAllEnabledServerIdsWithSSLPolicyIds 查找使用某个SSL策略的所有服务Id
func (this *ServerDAO) FindAllEnabledServerIdsWithSSLPolicyIds(tx *dbs.Tx, sslPolicyIds []int64) (result []int64, err error) {
	if len(sslPolicyIds) == 0 {
		return
	}

	for _, policyId := range sslPolicyIds {
		ones, err := this.Query(tx).
			State(ServerStateEnabled).
			ResultPk().
			Where("(JSON_CONTAINS(https, :jsonQuery) OR JSON_CONTAINS(tls, :jsonQuery))").
			Param("jsonQuery", maps.Map{"sslPolicyRef": maps.Map{"sslPolicyId": policyId}}.AsJSON()).
			FindAll()
		if err != nil {
			return nil, err
		}
		for _, one := range ones {
			serverId := int64(one.(*Server).Id)
			if !lists.ContainsInt64(result, serverId) {
				result = append(result, serverId)
			}
		}
	}
	return
}

// CountEnabledServersWithWebIds 计算使用某个缓存策略的所有服务数量
func (this *ServerDAO) CountEnabledServersWithWebIds(tx *dbs.Tx, webIds []int64) (count int64, err error) {
	if len(webIds) == 0 {
		return
	}
	return this.Query(tx).
		State(ServerStateEnabled).
		Attr("webId", webIds).
		Reuse(false).
		Count()
}

// FindAllEnabledServersWithWebIds 查找使用某个缓存策略的所有服务
func (this *ServerDAO) FindAllEnabledServersWithWebIds(tx *dbs.Tx, webIds []int64) (result []*Server, err error) {
	if len(webIds) == 0 {
		return
	}
	_, err = this.Query(tx).
		State(ServerStateEnabled).
		Attr("webId", webIds).
		Reuse(false).
		AscPk().
		Slice(&result).
		FindAll()
	return
}

// CountAllEnabledServersWithNodeClusterId 计算使用某个集群的所有服务数量
func (this *ServerDAO) CountAllEnabledServersWithNodeClusterId(tx *dbs.Tx, clusterId int64) (int64, error) {
	return this.Query(tx).
		State(ServerStateEnabled).
		Attr("clusterId", clusterId).
		Count()
}

// CountAllEnabledServersWithGroupId 计算使用某个分组的服务数量
func (this *ServerDAO) CountAllEnabledServersWithGroupId(tx *dbs.Tx, groupId int64) (int64, error) {
	return this.Query(tx).
		State(ServerStateEnabled).
		Where("JSON_CONTAINS(groupIds, :groupId)").
		Param("groupId", numberutils.FormatInt64(groupId)).
		Count()
}

// FindAllServerDNSNamesWithDNSDomainId 查询使用某个DNS域名的所有服务域名
func (this *ServerDAO) FindAllServerDNSNamesWithDNSDomainId(tx *dbs.Tx, dnsDomainId int64) ([]string, error) {
	clusterIds, err := SharedNodeClusterDAO.FindAllEnabledClusterIdsWithDNSDomainId(tx, dnsDomainId)
	if err != nil {
		return nil, err
	}
	if len(clusterIds) == 0 {
		return nil, nil
	}
	ones, err := this.Query(tx).
		State(ServerStateEnabled).
		Attr("isOn", true).
		Attr("clusterId", clusterIds).
		Result("dnsName").
		Reuse(false). // 避免因为IN语句造成内存占用过多
		FindAll()
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, one := range ones {
		dnsName := one.(*Server).DnsName
		if len(dnsName) == 0 {
			continue
		}
		result = append(result, dnsName)
	}
	return result, nil
}

// FindAllServersDNSWithClusterId 获取某个集群下的服务DNS信息
func (this *ServerDAO) FindAllServersDNSWithClusterId(tx *dbs.Tx, clusterId int64) (result []*Server, err error) {
	_, err = this.Query(tx).
		State(ServerStateEnabled).
		Attr("isOn", true).
		Attr("isAuditing", false). // 不在审核中
		Attr("clusterId", clusterId).
		Result("id", "name", "dnsName").
		DescPk().
		Slice(&result).
		FindAll()
	return
}

// GenerateServerDNSName 重新生成子域名
func (this *ServerDAO) GenerateServerDNSName(tx *dbs.Tx, serverId int64) (string, error) {
	if serverId <= 0 {
		return "", errors.New("invalid serverId")
	}
	dnsName, err := this.GenDNSName(tx)
	if err != nil {
		return "", err
	}
	op := NewServerOperator()
	op.Id = serverId
	op.DnsName = dnsName
	err = this.Save(tx, op)
	if err != nil {
		return "", err
	}

	err = this.NotifyUpdate(tx, serverId)
	if err != nil {
		return "", err
	}

	err = this.NotifyDNSUpdate(tx, serverId)
	if err != nil {
		return "", err
	}

	return dnsName, nil
}

// FindServerClusterId 查询当前服务的集群ID
func (this *ServerDAO) FindServerClusterId(tx *dbs.Tx, serverId int64) (int64, error) {
	return this.Query(tx).
		Pk(serverId).
		Result("clusterId").
		FindInt64Col(0)
}

// FindServerDNSName 查询服务的DNS名称
func (this *ServerDAO) FindServerDNSName(tx *dbs.Tx, serverId int64) (string, error) {
	return this.Query(tx).
		Pk(serverId).
		Result("dnsName").
		FindStringCol("")
}

// FindStatelessServerDNS 查询服务的DNS相关信息，并且不关注状态
func (this *ServerDAO) FindStatelessServerDNS(tx *dbs.Tx, serverId int64) (*Server, error) {
	one, err := this.Query(tx).
		Pk(serverId).
		Result("id", "dnsName", "isOn", "state", "clusterId").
		Find()
	if err != nil || one == nil {
		return nil, err
	}
	return one.(*Server), nil
}

// FindServerAdminIdAndUserId 获取当前服务的管理员ID和用户ID
func (this *ServerDAO) FindServerAdminIdAndUserId(tx *dbs.Tx, serverId int64) (adminId int64, userId int64, err error) {
	one, err := this.Query(tx).
		Pk(serverId).
		Result("adminId", "userId").
		Find()
	if err != nil {
		return 0, 0, err
	}
	if one == nil {
		return 0, 0, nil
	}
	return int64(one.(*Server).AdminId), int64(one.(*Server).UserId), nil
}

// CheckUserServer 检查用户服务
func (this *ServerDAO) CheckUserServer(tx *dbs.Tx, userId int64, serverId int64) error {
	if serverId <= 0 || userId <= 0 {
		return ErrNotFound
	}
	ok, err := this.Query(tx).
		Pk(serverId).
		Attr("userId", userId).
		Exist()
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// UpdateUserServersClusterId 设置一个用户下的所有服务的所属集群
func (this *ServerDAO) UpdateUserServersClusterId(tx *dbs.Tx, userId int64, clusterId int64) error {
	// 之前的cluster
	oldClusterId, err := SharedUserDAO.FindUserClusterId(tx, userId)
	if err != nil {
		return err
	}
	if oldClusterId == clusterId {
		return nil
	}

	_, err = this.Query(tx).
		Attr("userId", userId).
		Set("clusterId", clusterId).
		Update()
	if err != nil {
		return err
	}

	if oldClusterId > 0 {
		err = SharedNodeTaskDAO.CreateClusterTask(tx, oldClusterId, NodeTaskTypeConfigChanged)
		if err != nil {
			return err
		}
		err = SharedNodeTaskDAO.CreateClusterTask(tx, oldClusterId, NodeTaskTypeIPItemChanged)
		if err != nil {
			return err
		}
	}

	if clusterId > 0 {
		err = SharedNodeTaskDAO.CreateClusterTask(tx, clusterId, NodeTaskTypeConfigChanged)
		if err != nil {
			return err
		}
		err = SharedNodeTaskDAO.CreateClusterTask(tx, clusterId, NodeTaskTypeIPItemChanged)
		if err != nil {
			return err
		}
	}

	return err
}

// FindAllEnabledServersWithUserId 查找用户的所有的服务
func (this *ServerDAO) FindAllEnabledServersWithUserId(tx *dbs.Tx, userId int64) (result []*Server, err error) {
	_, err = this.Query(tx).
		State(ServerStateEnabled).
		Attr("userId", userId).
		DescPk().
		Slice(&result).
		FindAll()
	return
}

// FindEnabledServerIdWithWebId 根据WebId查找ServerId
func (this *ServerDAO) FindEnabledServerIdWithWebId(tx *dbs.Tx, webId int64) (serverId int64, err error) {
	if webId <= 0 {
		return 0, nil
	}
	return this.Query(tx).
		State(ServerStateEnabled).
		Attr("webId", webId).
		ResultPk().
		FindInt64Col(0)
}

// FindEnabledServerIdWithReverseProxyId 查找包含某个反向代理的Server
func (this *ServerDAO) FindEnabledServerIdWithReverseProxyId(tx *dbs.Tx, reverseProxyId int64) (serverId int64, err error) {
	return this.Query(tx).
		State(ServerStateEnabled).
		Where("JSON_CONTAINS(reverseProxy, :jsonQuery)").
		Param("jsonQuery", maps.Map{"reverseProxyId": reverseProxyId}.AsJSON()).
		ResultPk().
		FindInt64Col(0)
}

// CheckPortIsUsing 检查端口是否被使用
func (this *ServerDAO) CheckPortIsUsing(tx *dbs.Tx, clusterId int64, port int, excludeServerId int64, excludeProtocol string) (bool, error) {
	listen := maps.Map{
		"portRange": strconv.Itoa(port),
	}
	query := this.Query(tx).
		Attr("clusterId", clusterId).
		State(ServerStateEnabled)
	protocols := []string{"http", "https", "tcp", "tls", "udp"}
	where := ""
	if excludeServerId <= 0 {
		conds := []string{}
		for _, p := range protocols {
			conds = append(conds, "JSON_CONTAINS("+p+", :listen, '$.listen')")
		}
		where = strings.Join(conds, " OR ")
	} else {
		conds := []string{}
		for _, p := range protocols {
			conds = append(conds, "JSON_CONTAINS("+p+", :listen, '$.listen')")
		}
		where1 := "(id!=:serverId AND (" + strings.Join(conds, " OR ") + "))"

		conds = []string{}
		for _, p := range protocols {
			if p == excludeProtocol {
				continue
			}
			conds = append(conds, "JSON_CONTAINS("+p+", :listen, '$.listen')")
		}
		where2 := "(id=:serverId AND (" + strings.Join(conds, " OR ") + "))"
		where = where1 + " OR " + where2
		query.Param("serverId", excludeServerId)
	}
	return query.
		Where("("+where+")").
		Param("listen", string(listen.AsJSON())).
		Exist()
}

// ExistServerNameInCluster 检查ServerName是否已存在
func (this *ServerDAO) ExistServerNameInCluster(tx *dbs.Tx, clusterId int64, serverName string, excludeServerId int64) (bool, error) {
	query := this.Query(tx).
		Attr("clusterId", clusterId).
		Where("(JSON_CONTAINS(serverNames, :jsonQuery1) OR JSON_CONTAINS(serverNames, :jsonQuery2))").
		Param("jsonQuery1", maps.Map{"name": serverName}.AsJSON()).
		Param("jsonQuery2", maps.Map{"subNames": serverName}.AsJSON())
	if excludeServerId > 0 {
		query.Neq("id", excludeServerId)
	}
	query.State(ServerStateEnabled)
	return query.Exist()
}

// GenDNSName 生成DNS Name
func (this *ServerDAO) GenDNSName(tx *dbs.Tx) (string, error) {
	for {
		dnsName := rands.HexString(8)
		exist, err := this.Query(tx).
			Attr("dnsName", dnsName).
			Exist()
		if err != nil {
			return "", err
		}
		if !exist {
			return dnsName, nil
		}
	}
}

// FindLatestServers 查询最近访问的服务
func (this *ServerDAO) FindLatestServers(tx *dbs.Tx, size int64) (result []*Server, err error) {
	itemTable := SharedLatestItemDAO.Table
	itemType := LatestItemTypeServer
	_, err = this.Query(tx).
		Result(this.Table+".id", this.Table+".name").
		Join(SharedLatestItemDAO, dbs.QueryJoinRight, this.Table+".id="+itemTable+".itemId AND "+itemTable+".itemType='"+itemType+"'").
		Asc("CEIL((UNIX_TIMESTAMP() - " + itemTable + ".updatedAt) / (7 * 86400))"). // 优先一个星期以内的
		Desc(itemTable + ".count").
		State(NodeClusterStateEnabled).
		Limit(size).
		Slice(&result).
		FindAll()
	return
}

// NotifyUpdate 同步集群
func (this *ServerDAO) NotifyUpdate(tx *dbs.Tx, serverId int64) error {
	// 更新配置
	_, err := this.RenewServerConfig(tx, serverId, true)
	if err != nil && err != ErrNotFound {
		return err
	}

	// 创建任务
	clusterId, err := this.FindServerClusterId(tx, serverId)
	if err != nil {
		return err
	}
	if clusterId == 0 {
		return nil
	}
	return SharedNodeTaskDAO.CreateClusterTask(tx, clusterId, NodeTaskTypeConfigChanged)
}

// NotifyDNSUpdate 通知DNS更新
func (this *ServerDAO) NotifyDNSUpdate(tx *dbs.Tx, serverId int64) error {
	clusterId, err := this.Query(tx).
		Pk(serverId).
		Result("clusterId").
		FindInt64Col(0) // 这里不需要加服务状态条件，因为我们即使删除也要删除对应的服务的DNS解析
	if err != nil {
		return err
	}
	if clusterId <= 0 {
		return nil
	}
	dnsInfo, err := SharedNodeClusterDAO.FindClusterDNSInfo(tx, clusterId)
	if err != nil {
		return err
	}
	if dnsInfo == nil {
		return nil
	}
	if len(dnsInfo.DnsName) == 0 || dnsInfo.DnsDomainId <= 0 {
		return nil
	}
	return dns.SharedDNSTaskDAO.CreateServerTask(tx, serverId, dns.DNSTaskTypeServerChange)
}
