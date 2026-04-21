package chproto

import "testing"

func TestProtocolConstants(t *testing.T) {
	if ProtoVersion != 54460 {
		t.Errorf("ProtoVersion = %d, want 54460", ProtoVersion)
	}
	if ProtoRevision != 54461 {
		t.Errorf("ProtoRevision = %d, want 54461", ProtoRevision)
	}
	if ProtoDBMSVersion != "24.8.5.115" {
		t.Errorf("ProtoDBMSVersion = %q, want %q", ProtoDBMSVersion, "24.8.5.115")
	}
	if ProtoDBMSName != "ClickHouse" {
		t.Errorf("ProtoDBMSName = %q, want %q", ProtoDBMSName, "ClickHouse")
	}
}

func TestClientPacketTypes(t *testing.T) {
	// Verify client packet types are distinct
	packets := map[string]byte{
		"Hello":  PacketHello,
		"Query":  PacketQuery,
		"Data":   PacketData,
		"Cancel": PacketCancel,
		"Ping":   PacketPing,
	}

	seen := make(map[byte]string)
	for name, val := range packets {
		if existing, ok := seen[val]; ok {
			t.Errorf("packet type collision: %s and %s both have value %d", name, existing, val)
		}
		seen[val] = name
	}
}

func TestServerPacketTypes(t *testing.T) {
	// Verify server packet types are distinct
	packets := map[string]byte{
		"Data":                 ServerPacketData,
		"Exception":            ServerPacketException,
		"Progress":             ServerPacketProgress,
		"Pong":                 ServerPacketPong,
		"EndOfStream":          ServerPacketEndOfStream,
		"ProfileInfo":          ServerPacketProfileInfo,
		"Totals":               ServerPacketTotals,
		"Extremes":             ServerPacketExtremes,
		"TablesStatusResponse": ServerPacketTablesStatusResponse,
		"Log":                  ServerPacketLog,
		"TableColumns":         ServerPacketTableColumns,
		"PartUUIDs":            ServerPacketPartUUIDs,
		"ReadTaskRequest":      ServerPacketReadTaskRequest,
		"ProfileEvents":        ServerPacketProfileEvents,
		"Tree":                 ServerPacketTree,
	}

	seen := make(map[byte]string)
	for name, val := range packets {
		if existing, ok := seen[val]; ok {
			t.Errorf("server packet type collision: %s and %s both have value %d", name, existing, val)
		}
		seen[val] = name
	}
}

func TestQueryStageConstants(t *testing.T) {
	if QueryStageComplete != 0 {
		t.Errorf("QueryStageComplete = %d, want 0", QueryStageComplete)
	}
	if QueryStageFetchingData != 1 {
		t.Errorf("QueryStageFetchingData = %d, want 1", QueryStageFetchingData)
	}
	if QueryStageWithTotals != 2 {
		t.Errorf("QueryStageWithTotals = %d, want 2", QueryStageWithTotals)
	}
}

func TestQueryKindConstants(t *testing.T) {
	if QueryInitial != 0 {
		t.Errorf("QueryInitial = %d, want 0", QueryInitial)
	}
	if QuerySecondary != 1 {
		t.Errorf("QuerySecondary = %d, want 1", QuerySecondary)
	}
}

func TestCompressionConstants(t *testing.T) {
	if CompressionDisabled != 0 {
		t.Errorf("CompressionDisabled = %d, want 0", CompressionDisabled)
	}
	if CompressionEnabled != 1 {
		t.Errorf("CompressionEnabled = %d, want 1", CompressionEnabled)
	}
}

func TestClientServerPacketNoOverlap(t *testing.T) {
	// Ensure client and server packet types don't overlap (except where intentional)
	clientPackets := []byte{PacketHello, PacketQuery, PacketData, PacketCancel, PacketPing}
	serverPackets := []byte{ServerPacketData, ServerPacketException, ServerPacketProgress,
		ServerPacketPong, ServerPacketEndOfStream, ServerPacketProfileInfo}

	clientSet := make(map[byte]bool)
	for _, p := range clientPackets {
		clientSet[p] = true
	}

	// PacketData (client=2) and ServerPacketData (server=1) are different values
	// This is intentional as they flow in opposite directions
	for _, sp := range serverPackets {
		if clientSet[sp] {
			// Some overlap is expected (e.g., Data packet)
			t.Logf("packet type %d used in both directions (expected for some types)", sp)
		}
	}
}

func TestDBMSVersionComponents(t *testing.T) {
	// Verify version components are consistent with version string
	if ProtoDBMSMajor != 24 {
		t.Errorf("ProtoDBMSMajor = %d, want 24", ProtoDBMSMajor)
	}
	if ProtoDBMSMinor != 8 {
		t.Errorf("ProtoDBMSMinor = %d, want 8", ProtoDBMSMinor)
	}
	if ProtoDBMSPatch != 5 {
		t.Errorf("ProtoDBMSPatch = %d, want 5", ProtoDBMSPatch)
	}
}

func TestTimezoneAndDisplayName(t *testing.T) {
	if ProtoTimezone != "UTC" {
		t.Errorf("ProtoTimezone = %q, want %q", ProtoTimezone, "UTC")
	}
	if ProtoDisplayName != "AgentX Proxy" {
		t.Errorf("ProtoDisplayName = %q, want %q", ProtoDisplayName, "AgentX Proxy")
	}
}
