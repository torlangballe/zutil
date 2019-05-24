package zmpeg

import (
	"fmt"
	"io"
	"os"

	"github.com/Comcast/gots/packet"
)

func GetPacketsFromFile(filepath string) (packets []*packet.Packet, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Println("Unable to open file:", err, filepath)
		return
	}
	for {
		var pkt packet.Packet
		p := pkt[:]
		read, ferr := file.Read(p)
		if read == 0 {
			break
		}
		if ferr != nil {
			if ferr != io.EOF {
				err = ferr
				fmt.Println("GetPacketsFromUrl:", err)
			}
			return
		}
		packets = append(packets, &pkt)
	}
	if err == io.EOF {
		err = nil
	}
	return
}
