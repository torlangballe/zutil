package zprocess

const launchAgentPlistStr = `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
	<dict>
		<key>Label</key>
		<string>{id}</string>
		<key>RunAtLoad</key>
		<true/>
		<key>KeepAlive</key>
		<true/>
		<key>SoftResourceLimits</key>
		<dict>
			<key>NumberOfFiles</key>
			<integer>2048</integer>
		</dict>
        <key>StandardOutPath</key>
        <string>{var}/log.txt</string>
		<key>ProcessType</key>
		<string>Interactive</string>
		<key>Nice</key>
		<integer>-10</integer>
        <key>StandardErrorPath</key>
        <string>{var}/log.txt</string>
        <key>ProgramArguments</key>
        <array>
			<string>{bin-path}</string>
			{args}
        </array>
        <key>WorkingDirectory</key>
        <string>{binpath}/</string>
	</dict>
</plist>`
