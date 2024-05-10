//go:build zui

package zfilelister

import (
	"slices"
	"strings"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zgridlist"
	"github.com/torlangballe/zui/zimageview"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
)

type FileListerView struct {
	zcontainer.StackView
	DirOptions

	DirFunc      func(dirOpts DirOptions, got func(paths []string, err error))
	GetImageURL  func(path string) string
	CurrentPaths []string // ends in / if folders

	back       *zimageview.ImageView
	title      *zlabel.Label
	errorLabel *zlabel.Label
	grid       *zgridlist.GridListView
	rpcClient  *zrpc.Client
}

const (
	iconID  = "icon"
	titleID = "title"
	checkID = "check"
)

func makeLabel(text string) *zlabel.Label {
	label := zlabel.New(text)
	return label
}

func NewFileListerView(opts DirOptions, rpcClient *zrpc.Client) *FileListerView {
	v := &FileListerView{}

	v.SetMarginS(zgeo.SizeBoth(5))
	v.rpcClient = rpcClient
	v.Init(v, true, opts.StoreName+".FileLister")

	if opts.IconSize.IsNull() {
		opts.IconSize = zgeo.SizeD(24, 16)
	}
	v.DirOptions = opts
	v.DirOptions.PickedPaths = slices.Clone(opts.PickedPaths)
	bar := zcontainer.StackViewHor("bar")
	v.Add(bar, zgeo.TopLeft|zgeo.HorExpand)

	v.back = zimageview.New(nil, true, "images/triangle-left-gray.png", zgeo.SizeD(16, 16))
	bar.Add(v.back, zgeo.TopLeft)
	v.back.SetPressedHandler(v.goBack)

	v.title = makeLabel("")
	v.title.SetFont(zgeo.FontNice(zgeo.FontDefaultSize+3, zgeo.FontStyleBold))
	bar.Add(v.title, zgeo.TopLeft|zgeo.HorExpand)

	v.grid = zgridlist.NewView(opts.StoreName)
	v.grid.MakeFullSize = true
	v.grid.MaxColumns = 1
	v.grid.SetMinSize(zgeo.SizeD(340, 400))
	v.grid.MinRowsForFullSize = 5
	v.grid.MaxRowsForFullSize = 20
	v.grid.CellCountFunc = func() int {
		return len(v.CurrentPaths)
	}
	v.grid.CreateCellFunc = v.createRow
	v.grid.UpdateCellFunc = v.updateRow
	v.grid.HandleRowPressedFunc = v.handleRowPressed
	v.Add(v.grid, zgeo.TopLeft|zgeo.Expand)

	v.errorLabel = zlabel.New("")
	v.errorLabel.SetMaxWidth(300)
	v.errorLabel.Columns = 2
	v.errorLabel.SetColor(zgeo.ColorRed)
	v.Add(v.errorLabel, zgeo.TopLeft|zgeo.HorExpand)
	// zlog.Info("NewFileListerView:", v.DirOptions.PickedPaths)
	return v
}

func (v *FileListerView) goBack() {
	zlog.Assert(v.DirOptions.PathStub != "")
	// zlog.Info("goBack:", v.DirOptions.PathStub)
	var rest string
	v.DirOptions.PathStub = zstr.HeadUntilLast(v.DirOptions.PathStub, "/", &rest)
	if rest == "" {
		v.DirOptions.PathStub = ""
	}
	// zlog.Info("goBack2:", v.DirOptions.PathStub)
	v.updatePage()
}

func (v *FileListerView) handleRowPressed(id string) bool {
	path := v.pathOfID(id)
	name := strings.TrimRight(path, "/")
	if name == path { // it isn't a folder
		return false
	}
	v.DirOptions.PathStub = zfile.JoinPathParts(v.DirOptions.PathStub, name)
	// zlog.Info("pressed:", id, path, v.DirOptions.PathStub)
	v.updatePage()
	return true
}

func (v *FileListerView) ReadyToShow(beforeWindow bool) {
	if beforeWindow {
		v.updatePage()
	}
}

func (v *FileListerView) pathOfID(id string) string {
	index := v.grid.IndexOfID(id)
	return v.CurrentPaths[index]
}

func (v *FileListerView) updateRow(grid *zgridlist.GridListView, id string) {
	row := v.grid.CellView(id).(*zcontainer.StackView)
	path := v.pathOfID(id)

	fullFolderPath := zfile.JoinPathParts(v.PathStub, path)
	f, _ := row.FindViewWithName(iconID, false)
	iv := f.(*zimageview.ImageView)
	iurl := v.GetImageURL(fullFolderPath)
	// zlog.Info("updateRow image", path, fullFolderPath, iurl)
	iv.SetImage(nil, iurl, nil)

	f, _ = row.FindViewWithName(titleID, false)
	label := f.(*zlabel.Label)
	isFolder := (zstr.LastByteAsString(path) == "/")
	path = strings.TrimRight(path, "/")
	fullpath := zfile.JoinPathParts(v.PathStub, path)
	label.SetText(path)

	f, _ = row.FindViewWithName(checkID, false)
	check := f.(*zcheckbox.CheckBox)
	on := zbool.False
	for _, p := range v.DirOptions.PickedPaths {
		// zlog.Info("updateRow", fullFolderPath, p)
		if p == fullFolderPath {
			if on.IsFalse() {
				// zlog.Info("updateRow on", id)
				on = zbool.True
			}
		} else if isFolder && strings.HasPrefix(p, fullpath) { // !on.IsTrue() &&
			// zlog.Info("updateRow udef", id)
			on = zbool.Unknown
		}
	}
	// zlog.Info("updateRow set", fullFolderPath, on)
	check.SetValue(on)
}

func (v *FileListerView) createRow(grid *zgridlist.GridListView, id string) zview.View {
	s := zcontainer.StackViewHor(id)
	s.SetMarginS(zgeo.SizeD(4, 3))
	s.SetSpacing(6)

	check := zcheckbox.New(zbool.False)
	check.SetObjectName(checkID)
	check.SetValueHandler(func() {
		on := check.On()
		path := v.pathOfID(id)
		isFolder := (zstr.LastByteAsString(path) == "/")
		fullpath := zfile.JoinPathParts(v.PathStub, path)
		zslice.DeleteFromFunc(&v.DirOptions.PickedPaths, func(path string) bool {
			if path == fullpath {
				return !on // remove self if just turned off
			}
			if strings.HasPrefix(fullpath, path) {
				return true
			}
			if isFolder && strings.HasPrefix(path, fullpath) {
				zlog.Info("DelChild:", path, fullpath)
				return true
			}
			return false
		})
		if on {
			v.DirOptions.PickedPaths = append(v.DirOptions.PickedPaths, fullpath) // we can add it, as it will be removed above
		}
	})
	s.Add(check, zgeo.CenterLeft)

	icon := zimageview.New(nil, true, "", v.IconSize)
	icon.SetObjectName(iconID)
	s.Add(icon, zgeo.CenterLeft)

	title := makeLabel("")
	title.SetObjectName(titleID)
	s.Add(title, zgeo.CenterLeft|zgeo.HorExpand)

	return s
}

func (v *FileListerView) updatePage() {
	title := zfile.JoinPathParts(v.DirOptions.StoreName, v.DirOptions.PathStub)
	// zlog.Info("update:", title)
	v.title.SetText(title)
	v.back.SetUsable(v.DirOptions.PathStub != "")
	v.DirFunc(v.DirOptions, func(paths []string, err error) {
		if zlog.OnError(err, "DirFunc", v.DirOptions.PathStub) {
			return
		}
		v.CurrentPaths = paths
		v.grid.LayoutCells(true)
	})
}

// func MakeGetURL(storeName, urlPrefix, urlStub, toToken string) string {
// 	if !zhttp.StringStartsWithHTTPX(urlPrefix) {
// 		urlPrefix = zfile.JoinPathParts(zapp.URLStub(), zrest.AppURLPrefix)
// 	}
// 	if urlStub == "" {
// 		urlStub = "zfilelister-files/" + storeName
// 	}
// 	basePrefix := zfile.JoinPathParts(urlPrefix, urlStub)
// 	if toToken != "" {
// 		basePrefix += "?token=" + toToken
// 	}
// 	return basePrefix
// }

func NewRemoteFileListerView(urlPrefix, urlStub string, opts DirOptions, rpcClient *zrpc.Client) *FileListerView {
	flister := NewFileListerView(opts, rpcClient)
	// zlog.Info("NewRemoteFileListerView.rpcClient:", flister.rpcClient != nil)
	flister.DirFunc = func(dirOpts DirOptions, got func(paths []string, err error)) {
		go func() {
			var paths []string
			err := flister.rpcClient.Call("FileServerCalls.GetDirectory", dirOpts, &paths)
			// zlog.Info("NewRemoteFileListerView.GetDir:", paths, err)
			if err != nil {
				got(nil, err)
				return
			}
			got(paths, nil)
		}()
	}
	// zlog.Info("NewRemoteFileLister: DirFunc:", flister.DirFunc != nil)
	flister.GetImageURL = func(spath string) string {
		// ext := path.Ext(spath)
		// if zstr.HasSuffix(spath, "/", &spath) {
		// 	ext = "._folder"
		// }
		surl := zfile.JoinPathParts(urlPrefix, cachePrefix, opts.StoreName, spath)
		surl += "?size=" + flister.IconSize.String()
		// zlog.Info("RemoteFileLister:Image", surl)
		return surl
	}
	return flister
}

func (v *FileListerView) Present(title string, got func(pickedPaths []string)) {
	att := zpresent.ModalDialogAttributes
	zalert.PresentOKCanceledView(v, title, att, nil, func(ok bool) bool {
		if ok {
			got(v.DirOptions.PickedPaths)
		}
		return true
	})
}
