package memory

import (
	"encoding/binary"
	"log"

	"github.com/digitalocean/go-smbios/smbios"
)

type Memory struct {
	Bank         string `json:"bank"`
	Size         int    `json:"size"`
	Unit         string `json:"unit"`
	Type         string `json:"type"`
	FormFactor   string `json:"formFactor"`
	Manufacturer string `json:"manufacturer"`
	Serial       string `json:"serial"`
	AssetTag     string `json:"assetTag"`
	PartNumber   string `json:"partNumber"`
	Speed        int    `json:"speed"`
	DataWidth    int    `json:"dataWidth"`
	TotalWidth   int    `json:"totalWidth"`
}

func formFactorType(ff int) string {
	return [...]string{"",
		"Other",
		"Unknown",
		"SIMM",
		"SIP",
		"Chip",
		"DIP",
		"ZIP",
		"Proprietary Card",
		"DIMM",
		"TSOP",
		"Row of Chips",
		"RIMM",
		"SODIMM",
		"SRIMM",
		"FBDIMM"}[ff]
}

func memoryType(mt int) string {
	return [...]string{"",
		"Other",
		"Unknown",
		"DRAM",
		"EDRAM",
		"VRAM",
		"SRAM",
		"RAM",
		"ROM",
		"FLASH",
		"EEPROM",
		"FEPROM",
		"EPROM",
		"CDRAM",
		"3DRAM",
		"SDRAM",
		"SGRAM",
		"RDRAM",
		"DDR",
		"DDR2",
		"DDR2 FB-DIMM",
		"Reserved",
		"Reserved",
		"Reserved",
		"DDR3",
		"FBD2",
		"DDR4",
		"LPDDR",
		"LPDDR2",
		"LPDDR3",
		"LPDDR4"}[mt]
}

func Info() []Memory {
	var mems []Memory
	rc, _, err := smbios.Stream()
	if err != nil {
		log.Println("failed to open stream: ", err)
		return []Memory{}

	}

	defer rc.Close()

	d := smbios.NewDecoder(rc)
	ss, err := d.Decode()
	if err != nil {
		log.Println("failed to decode structures: ", err)
		return []Memory{}
	}

	for _, s := range ss {
		if s.Header.Type != 17 {
			continue
		}

		size := int(binary.LittleEndian.Uint16(s.Formatted[8:10]))
		bankLocator := s.Strings[0]

		if size == 0 {
			bankLocator = "empty"
			continue
		}

		if size == 0x7fff {
			size = int(binary.LittleEndian.Uint32(s.Formatted[24:28]))
		}

		manufacturer := s.Strings[1]
		serial := s.Strings[2]
		assetTag := s.Strings[3]
		partNumber := s.Strings[4]

		totalWidth := s.Formatted[4]
		dataWidth := s.Formatted[6]
		formFactorBytes := s.Formatted[10]
		memTypeBytes := s.Formatted[14]
		memType := memoryType(int(memTypeBytes))
		formFactor := formFactorType(int(formFactorBytes))
		memSpeed := int(binary.LittleEndian.Uint16(s.Formatted[17:19]))

		unit := "KB"
		if s.Formatted[9]&0x80 == 0 {
			unit = "MB"
		}

		mems = append(mems, Memory{
			Bank:         bankLocator,
			Size:         size,
			Unit:         unit,
			Type:         memType,
			FormFactor:   formFactor,
			Manufacturer: manufacturer,
			Serial:       serial,
			AssetTag:     assetTag,
			PartNumber:   partNumber,
			Speed:        memSpeed,
			DataWidth:    int(dataWidth),
			TotalWidth:   int(totalWidth),
		})
	}
	return mems
}
