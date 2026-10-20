on run argv
    set targetFile to POSIX file (item 1 of argv) as alias
    tell application "Finder"
        activate
        open information window of targetFile
    end tell
end run