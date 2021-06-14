package main

import (
	"os"
	"strings"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
)

func doCommand(command string, print, fail bool, args ...string) string {
	str := strings.Join(append([]string{command}, args...), " ")
	str, err := zprocess.RunBashCommand(str, 0)
	// zlog.Info("RUNCOMMAND:", err)
	if err != nil {
		zlog.Info(err, zlog.StackAdjust(1), str)
		if fail {
			os.Exit(-1)
		}
		return str
	}
	zlog.Info("build:", str)

	if print && str != "" {
		zlog.Info(str)
	}
	return str
}

func main() {
	os.Mkdir("certs", 0775|os.ModeDir)
	certName := os.Args[1]
	// Key considerations for algorithm "RSA" ≥ 2048-bit
	doCommand("🟨openssl", true, true, "genrsa -out certs/"+certName+".key 2048")
	// Key considerations for algorithm "ECDSA" ≥ secp384r1
	// List ECDSA the supported curves (openssl ecparam -list_curves)
	doCommand("🟨openssl", true, true, "ecparam -genkey -name secp384r1 -out certs/"+certName+".key")
	//	Generation of self-signed(x509) public key (PEM-encodings .pem|.crt) based on the private (.key)
	doCommand("🟨openssl", true, true, "req -new -x509 -sha256 -key certs/"+certName+".key -out certs/"+certName+".crt -days 3650")

}
