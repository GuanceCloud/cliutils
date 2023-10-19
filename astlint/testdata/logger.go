package testdata

import "github.com/GuanceCloud/cliutils/logger"

func main() {
	l := logger.SLogger("testdata")

	f := func() error {
		return nil
	}

	l.Debug("debug")
	l.Debugf("debugf: %f", 3.14)

	l.Info("info")
	l.Infof("infof: %f", 1.414)

	l.Infof("infof: %v", f)
}
