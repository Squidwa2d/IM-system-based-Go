package util

const (
	PC     = "PC"
	MOBILE = "MOBILE"
)

func IsSupportedDevice(Device string) bool {
	//check if the currency is supported
	switch Device {
	case PC, MOBILE:
		return true
	}
	return false
}
