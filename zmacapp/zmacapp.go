package zmacapp

import (
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
)

func makeFolder(parent, folder string) string {
	folder = zfile.ExpandTildeInFilepath(folder)
	if parent != "" {
		parent = zfile.ExpandTildeInFilepath(parent)
		folder = zfile.JoinPathParts(parent, folder)
	}
	zfile.MakeDirAllIfNotExists(folder)
	return folder
}

func MakeApp(binaryPath, bundleID, appPathWithName string, args []string) error {
	appPathWithName = zfile.ChangedExtension(appPathWithName, ".app")
	appPathWithName = makeFolder("", appPathWithName)
	contents := makeFolder(appPathWithName, "Contents")
	macos := makeFolder(contents, "MacOS")

	binaryPath = zfile.ExpandTildeInFilepath(binaryPath)
	_, name, _, _ := zfile.Split(binaryPath)
	binaryDest := zfile.JoinPathParts(macos, name)
	err := zfile.CopyFile(binaryDest, binaryPath)
	if zlog.OnError(err) {
		return err
	}
	plist := zfile.JoinPathParts(contents, "Info.plist")
	err = WriteAppPList(plist, name, bundleID, args)
	if zlog.OnError(err) {
		return err
	}
	return nil
}

// func makeAppIcons(appFolder string) error {
// 	// start by copying the icon into the bundle
// 	iconFilename := filepath.Base(iconFile)
// 	resFolder := filepath.Join(appFolder, "Contents", "Resources")
// 	copyTo := filepath.Join(resFolder, iconFilename)
// 	err := copyFile(iconFile, copyTo, nil)
// 	if err != nil {
// 		return err
// 	}

// 	useIcon := iconFile // usable icon files are of type .png, .jpg, .gif, or .tiff - and we handle .svg
// 	tmpFolder := filepath.Join(resFolder, "tmp")
// 	err = os.MkdirAll(tmpFolder, 0755)
// 	if err != nil {
// 		return err
// 	}
// 	defer os.RemoveAll(tmpFolder)

// 	// lazy way to convert SVG files to PNG, by using QuickLook
// 	// -z displays generation performance info (instead of showing thumbnail)
// 	// -t Computes the thumbnail
// 	// -s sets the size of the thumbnail
// 	// -o sets the output directory (NOT the actual output file)
// 	if filepath.Ext(iconFile) == ".svg" {
// 		cmd := exec.Command("qlmanage", "-z", "-t", "-s", "1024", "-o", tmpFolder, iconFile)
// 		cmd.Stdout = os.Stdout
// 		cmd.Stderr = os.Stderr
// 		err := cmd.Run()
// 		if err != nil {
// 			return fmt.Errorf("running qlmanage: %v", err)
// 		}
// 		useIcon = filepath.Join(tmpFolder, iconFile+".png")
// 	}

// 	// make the various icon sizes
// 	// see https://developer.apple.com/library/content/documentation/GraphicsAnimation/Conceptual/HighResolutionOSX/Optimizing/Optimizing.html
// 	iconset := filepath.Join(tmpFolder, "icon.iconset")
// 	err = os.Mkdir(iconset, 0755)
// 	if err != nil {
// 		return err
// 	}
// 	sizes := []int{16, 32, 64, 128, 256, 512, 1024}
// 	for i, size := range sizes {
// 		nameSize := size
// 		var suffix string
// 		if i > 0 {
// 			nameSize = sizes[i-1]
// 			suffix = "@2x"
// 		}

// 		iconName := fmt.Sprintf("icon_%dx%d%s.png", nameSize, nameSize, suffix)
// 		outIconFile := filepath.Join(iconset, iconName)

// 		sizeStr := fmt.Sprintf("%d", size)
// 		cmd := exec.Command("sips", "-z", sizeStr, sizeStr, useIcon, "--out", outIconFile)
// 		cmd.Stdout = os.Stdout
// 		cmd.Stderr = os.Stderr
// 		err := cmd.Run()
// 		if err != nil {
// 			return fmt.Errorf("running sips: %v", err)
// 		}

// 		// make standard-DPI version if we didn't already
// 		if i > 0 && i < len(sizes)-1 {
// 			stdName := fmt.Sprintf("icon_%dx%d.png", size, size)
// 			err := copyFile(outIconFile, filepath.Join(iconset, stdName), nil)
// 			if err != nil {
// 				return fmt.Errorf("copying icon file: %v", err)
// 			}
// 		}
// 	}

// 	// create the final .icns file
// 	icnsFile := filepath.Join(resFolder, "icon.icns")
// 	cmd := exec.Command("iconutil", "-c", "icns", "-o", icnsFile, iconset)
// 	cmd.Stdout = os.Stdout
// 	cmd.Stderr = os.Stderr
// 	err = cmd.Run()
// 	if err != nil {
// 		return fmt.Errorf("running iconutil: %v", err)
// 	}

// 	return nil
// }
