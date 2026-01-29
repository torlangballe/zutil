//go:build zui

package zrpc

import "github.com/torlangballe/zui/zview"

func (c *Client) CallToDownload(method, filename string, input any) error {
	var path string
	err := c.Call(method, input, &path)
	if err != nil {
		return err
	}
	surl := path
	zview.DownloadURI(surl, filename)
	return nil
}
