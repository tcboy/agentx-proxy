// Package chproto implements ClickHouse Native (TCP) protocol encoding/decoding
package chproto

// Protocol constants
const (
	ProtoVersion       = 54460
	ProtoRevision      = 54461
	ProtoDBMSVersion   = "24.8.5.115"
	ProtoDBMSName      = "ClickHouse"
	ProtoDBMSMajor     = 24
	ProtoDBMSMinor     = 8
	ProtoDBMSPatch     = 5
	ProtoTimezone      = "UTC"
	ProtoDisplayName   = "AgentX Proxy"
)

// Packet types (client -> server)
const (
	PacketHello       = 0
	PacketQuery       = 1
	PacketData        = 2
	PacketCancel      = 3
	PacketPing        = 4
	PacketTablesStatusRequest = 5
)

// Packet types (server -> client)
const (
	ServerPacketData       = 1
	ServerPacketException  = 2
	ServerPacketProgress   = 3
	ServerPacketPong       = 4
	ServerPacketEndOfStream = 5
	ServerPacketProfileInfo = 6
	ServerPacketTotals     = 7
	ServerPacketExtremes   = 8
	ServerPacketTablesStatusResponse = 9
	ServerPacketLog      = 10
	ServerPacketTableColumns = 11
	ServerPacketPartUUIDs = 12
	ServerPacketReadTaskRequest = 13
	ServerPacketProfileEvents = 14
	ServerPacketTree     = 15
)

// Query stage
const (
	QueryStageComplete = 0
	QueryStageFetchingData = 1
	QueryStageWithTotals = 2
)

// Query kind
const (
	QueryInitial    = 0
	QuerySecondary  = 1
)

// Compression
const (
	CompressionDisabled = 0
	CompressionEnabled  = 1
)
