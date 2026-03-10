package util

import "strings"

const (
	PC     = "PC"
	MOBILE = "MOBILE"
)

func IsSupportedDevice(Device string) bool {
	//check if the currency is supported
	Device = strings.ToUpper(Device)
	switch Device {
	case PC, MOBILE:
		return true
	}
	return false
}
