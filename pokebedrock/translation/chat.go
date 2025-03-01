package translation

import "github.com/df-mc/dragonfly/server/player/chat"

// MessageJoin ...
func MessageJoin(msg string) chat.Translation {
	return chat.Translate(str(msg), 1, "")
}

// MessageQuit ...
func MessageQuit(msg string) chat.Translation {
	return chat.Translate(str(msg), 1, "")
}

// MessageServerDisconnect ...
func MessageServerDisconnect(msg string) chat.Translation {
	return chat.Translate(str(msg), 0, "")
}
