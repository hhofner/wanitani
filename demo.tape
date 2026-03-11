Output demo.gif

Set Shell zsh
Set FontSize 16
Set Width 900
Set Height 500
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms

# Start the app
Type "./wanitani"
Enter
Sleep 1s

# Show the unauthenticated screen
Sleep 2s

# Type /help to show commands
Type "/"
Sleep 1s
Type "h"
Sleep 500ms
Type "elp"
Sleep 500ms
Enter
Sleep 2s

# Start /learn using tab selection
Type "/"
Sleep 500ms

# Tab to highlight /learn
Tab
Sleep 300ms
Tab
Sleep 300ms
Tab
Sleep 500ms

Enter
Sleep 3s

# Browse lesson - switch tabs
Tab
Sleep 1.5s
Tab
Sleep 1.5s
Tab
Sleep 1.5s

# Go to next item
Enter
Sleep 1.5s

# Browse tabs on second item
Tab
Sleep 1.5s
Tab
Sleep 1.5s

# Go to next item
Enter
Sleep 1.5s

# Go to quiz (last item, press enter)
Enter
Sleep 2s

# Answer meaning question
Type "big"
Sleep 500ms
Enter
Sleep 1.5s
Enter
Sleep 1s

# Answer reading question with romaji -> hiragana
Type "tai"
Sleep 1s
Enter
Sleep 1.5s
Enter
Sleep 1s

# Exit
Ctrl+C
