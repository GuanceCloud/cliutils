package ipipnet

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb"
	ipipnet "github.com/ipipdotnet/ipdb-go"
)

var (
	l           = logger.DefaultSLogger("ipipnet")
	_ ipdb.IPdb = (*IpIpNet)(nil)
)

type IpIpNet struct {
	cityDB   *ipipnet.City
	initErr  error
	language string
}

func (i *IpIpNet) Init(dataDir string, config map[string]string) {
	matches, err := filepath.Glob(filepath.Join(dataDir, "ipdb", "ipipnet", "*.ipdb"))
	if err != nil || len(matches) == 0 {
		i.initErr = fmt.Errorf("the *.ipdb file is not found in dir [%s]", filepath.Join(dataDir, "ipdb", "ipipnet"))
		return
	}
	i.cityDB, i.initErr = ipipnet.NewCity(matches[0])
	if i.initErr != nil {
		i.initErr = fmt.Errorf("unable to init ipipnet city: %w", i.initErr)
		return
	}
	languages := i.cityDB.Languages()
	if len(languages) == 0 {
		i.initErr = fmt.Errorf("no available languages supported by ipipnet")
		return
	}
	if langCfg := strings.ToUpper(strings.TrimSpace(config["language"])); langCfg != "" {
		for _, lang := range languages {
			if lang == langCfg {
				i.language = lang
				return
			}
		}
		l.Warnf("the configured language [%s] is not supported by ipipnet, use another instead", langCfg)
	}
	for _, lang := range languages {
		if lang == "EN" {
			i.language = lang // prefer English
			return
		}
	}
	i.language = languages[0]
}

func (i *IpIpNet) Geo(ip string) (*ipdb.IPdbRecord, error) {
	if i.initErr != nil {
		return nil, fmt.Errorf("ipipnet init error: %w", i.initErr)
	}

	info, err := i.cityDB.FindInfo(ip, i.language)
	if err != nil {
		return nil, fmt.Errorf("unable to find geo info of ip address [%s]: %w", ip, err)
	}

	latitude, _ := strconv.ParseFloat(info.Latitude, 32)
	Longitude, _ := strconv.ParseFloat(info.Longitude, 32)

	return &ipdb.IPdbRecord{
		Country:   info.CountryName,
		Region:    info.RegionName,
		City:      info.CityName,
		Isp:       info.IspDomain,
		Latitude:  float32(latitude),
		Longitude: float32(Longitude),
		Timezone:  info.Timezone,
		Areacode:  info.AreaCode,
	}, nil
}

func (i *IpIpNet) SearchIsp(string) string {
	return ""
}
