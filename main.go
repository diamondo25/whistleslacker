package main

import (
	"flag"
	"fmt"
	"github.com/nlopes/slack"
	"strings"
	"time"
)

var slackToken = ""
var revertToSingleChannelGuest = false
var oldChannelFormat = "%s-old"

func init() {
	flag.StringVar(
		&slackToken,
		"token",
		slackToken,
		"Token used for Slack API. Use the web token 'xoxs' as an admin for best experience.",
	)
	flag.BoolVar(
		&revertToSingleChannelGuest,
		"revert-to-single-channel-guest",
		revertToSingleChannelGuest,
		"Single-channel users need to be swapped to a multi-channel guest in order to move them. "+
			"With this setting they'll get converted back to single-channel when they are moved. "+
			"Note: they'll not be able to access the old channel anymore",
	)
	flag.StringVar(
		&oldChannelFormat,
		"old-channel-format",
		oldChannelFormat,
		"Formatting to apply on the old channel name. "+
			"Note that the maximum channel name length is 21 characters!",
	)
}

var slackClient *slack.Client

func main() {
	flag.Parse()

	if !strings.HasPrefix(slackToken, "xoxs-") {
		fmt.Println("Tokens that do not start with xoxs- don't seem to work with changing user to multi-channel guest!")
	}

	slackClient = slack.New(slackToken)

	channels, _, err := slackClient.GetConversations(&slack.GetConversationsParameters{
		Types: []string{"private_channel"},
	})

	if err != nil {
		panic(err)
	}

	onlyChannels := flag.Args()

	for _, channel := range channels {
		if !channel.IsPrivate {
			continue
		}

		if len(onlyChannels) > 0 {
			skipChannel := true
			for _, v := range onlyChannels {
				if v == channel.Name {
					skipChannel = false
					break
				}
			}

			if skipChannel {
				continue
			}

		}

		fmt.Println("Converting channel:", channel.Name)

		err := migrateChannel(channel)
		if err != nil {
			fmt.Println(fmt.Sprintf("Error while converting %s: %v", channel.Name, err))
		} else {
			fmt.Println("Converted channel!")
		}
	}
}

var usersLoaded = make(map[string]*slack.User)

func migrateChannel(channel slack.Channel) error {
	allMembers, _, err := slackClient.GetUsersInConversation(&slack.GetUsersInConversationParameters{
		ChannelID: channel.ID,
	})
	if err != nil {
		panic(err)
	}

	oldChannelName := fmt.Sprintf(oldChannelFormat, channel.Name)

	_, err = slackClient.RenameConversation(channel.ID, oldChannelName)
	if err != nil {
		return fmt.Errorf("error while renaming conversation to %s: %v", oldChannelName, err)
	}

	newChannel, err := slackClient.CreateConversation(channel.Name, false)
	if err != nil {
		return fmt.Errorf("error while creating conversation: %v", err)
	}

	if channel.Purpose.Value != "" {
		_, err = slackClient.SetPurposeOfConversation(newChannel.ID, channel.Purpose.Value)
		if err != nil {
			return fmt.Errorf("error while setting purpose of conversation: %v", err)
		}
	}

	// Check which ones are single-channel
	for _, member := range allMembers {

		if _, k := usersLoaded[member]; k {
			// Already migrated user
			continue
		}

		memberInfo, err := slackClient.GetUserInfo(member)
		if err != nil {
			return fmt.Errorf("error getting user info from %s: %v", member, err)
		}

		usersLoaded[member] = memberInfo

		if memberInfo.IsUltraRestricted {
			fmt.Println(memberInfo.ID, " (", memberInfo.RealName, ") is a single-channel guest")
			err = slackClient.SetRestricted(memberInfo.TeamID, memberInfo.ID)
			if err != nil {
				fmt.Println("Unable to make", memberInfo.RealName, "a multi-channel guest:", err)
				continue
			} else {
				fmt.Println("Not anymore!")
			}

			if revertToSingleChannelGuest {
				// Make sure we turn them back
				defer func() {
					err := slackClient.SetUltraRestricted(memberInfo.TeamID, memberInfo.ID, newChannel.ID)
					if err != nil {
						fmt.Println("Unable to reset user ", memberInfo.ID, ":", err)
					} else {
						fmt.Println("Put back", memberInfo.Name, "as a single-channel guest")
					}
				}()
			}
		}
	}

	// Possible fix for Slack kicking the previously single-channel users from the old channel?
	time.Sleep(5 * time.Second)

	var usersToInvite []string

	for _, v := range allMembers {
		// Make sure we are not inviting ourselves
		if v == newChannel.Creator {
			continue
		}

		usersToInvite = append(usersToInvite, v)
	}

	_, err = slackClient.InviteUsersToConversation(newChannel.ID, usersToInvite...)

	if err != nil {
		return fmt.Errorf("error while inviting a list of people back into %s: %v", newChannel.Name, err)
	}

	return nil
}
