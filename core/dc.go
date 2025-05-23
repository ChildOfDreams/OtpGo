package core

import (
	"fmt"
	"otpgo/dc"
)

var DC dc.DCFile = dc.NewDCFile()

func LoadDC() (err error) {
	if Config.General.DC_Disable_Multiple_Inheritance {
		DC.SetMultipleInheritance(false)
	}
	if Config.General.DC_Disable_Virtual_Inheritance {
		DC.SetVirtualInheritance(false)
	}
	if Config.General.DC_Disable_Sort_Inheritance_By_File {
		DC.SetSortInheritanceByFile(false)
	}
	for _, conf := range Config.General.DC_Files {
		ok := DC.Read(conf)
		if !ok {
			return fmt.Errorf("failed to read DC file %s: %v", conf, err)
		}
	}
	return nil
}
