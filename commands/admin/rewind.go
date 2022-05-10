package admin

import (
	"ChatWire/fact"
	"ChatWire/glob"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

/* Load a different save-game */
func Rewind(s *discordgo.Session, i *discordgo.InteractionCreate) {

	var args []string = strings.Split("", " ")
	argnum := len(args)

	/* Correct number of arguments (1) */
	if argnum == 1 {

		glob.VoteBoxLock.Lock()
		glob.VoteBox.LastRewindTime = time.Now()
		fact.VoidAllVotes()     /* Void all votes */
		fact.WriteRewindVotes() /* Save to file before exiting */
		glob.VoteBoxLock.Unlock()

		fact.DoRewindMap(s, args[0])
	} else {
		fact.ShowRewindList(s)
		return
	}
}
