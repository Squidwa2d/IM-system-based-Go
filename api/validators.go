package api

import (
	util "github.com/Squidwa2d/IM-system-based-Go/utils"
	"github.com/go-playground/validator/v10"
)

var validDevice validator.Func = func(fieldLever validator.FieldLevel) bool {
	if Device, ok := fieldLever.Field().Interface().(string); ok {
		return util.IsSupportedDevice(Device)
	}
	return false
}
