package zcommand

import "fmt"

func CloseUrlInBrowser(surl string, btype BrowserType) error {
	command := `
		display alert "hello"
		repeat with w in windows
			repeat with t in tabs of w
				set u to URL of t
				if u is equal to "%s" then 
					close t
					return
				end if
           end repeat
       end repeat
	`
	command = fmt.Sprintf(command, surl)
	_, err := RunAppleScript(command, 10.0)
	return err
}

func CloseAppWindowWithTitle(app, title string, allExcept bool) error {
	titleName := "title"
	if app == GetAppNameOfBrowser(Safari, true) {
		titleName = "name"
	}
	command :=
		`tell application "%s"
		activate
		close (every window whose %s is %s "%s")
		end tell
`
	not := ""
	if allExcept {
		not = "not"
	}
	command = fmt.Sprintf(command, app, titleName, not, title)
	// fmt.Println("CloseWindowWithTitle:", "\n", command)
	_, err := RunAppleScript(command, 5.0)
	// fmt.Println("CloseWindowWithTitle done", err)
	return err
}
