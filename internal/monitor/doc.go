// Package monitor defines the read-only domain contract used by RC Monitor.
//
// The package intentionally contains no Modbus, serial, TCP bridge or controller
// register-map logic. Transport remains an RC Gateway responsibility and protocol
// interpretation remains a Rapid SCADA/provider responsibility.
package monitor
