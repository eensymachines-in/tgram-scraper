package models

import "encoding/json"

type Sender struct {
	SenderID  json.Number `json:"id"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	UName     string      `json:"username"`
}
type Chat struct {
	ChatID json.Number `json:"id"`
	Typ    string      `json:"type"`
}
type UpdateMessage struct {
	MsgId json.Number `json:"message_id"`
	From  Sender      `json:"from"`
	Chat  Chat        `json:"chat"`
	Text  string      `json:"text"`
}

type Update struct {
	UpdtID  json.Number   `json:"update_id"` //easier to deal with this as string, since its a big.Int, unless ofcourse you have a math operation on it
	Message UpdateMessage `json:"message"`
}

type UpdateResponse struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}
