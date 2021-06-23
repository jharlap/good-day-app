package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/zeebo/xxh3"
)

var (
	sapi          *slack.Client
	signingSecret string
)

func main() {
	signingSecret = os.Getenv("SLACK_SIGNING_SECRET")
	sapi = slack.New(os.Getenv("SLACK_BOT_TOKEN"))

	http.HandleFunc("/", printBody)
	http.Handle("/event", verifySecret(http.HandlerFunc(handleEvent)))
	http.Handle("/interactive", verifySecret(http.HandlerFunc(handleInteractive)))
	http.ListenAndServe(":3000", nil)
}

func verifySecret(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		sv, err := slack.NewSecretsVerifier(r.Header, signingSecret)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := sv.Write(body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if err := sv.Ensure(); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		next.ServeHTTP(w, r)
	})
}

func handleInteractive(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Debug().Err(err).Msg("error parsing interactive form")
		return
	}

	body := []byte(r.PostFormValue("payload"))
	var ic slack.InteractionCallback
	err = json.Unmarshal(body, &ic)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Debug().Err(err).Str("body", string(body)).Msg("error unmarshaling interactive body")
		return
	}

	if ic.Type == slack.InteractionTypeBlockActions && len(ic.ActionCallback.BlockActions) > 0 && ic.ActionCallback.BlockActions[0].ActionID == homeButtonStartReflection {
		startReflectionDialog(ic.TriggerID)
	} else if ic.Type == slack.InteractionTypeViewSubmission && ic.View.CallbackID == reflectionModalCallbackID {
		fmt.Printf("Modal Submission: %+v\n", ic)
		fmt.Println("first:", ic.View.State.Values["first-name-block"]["first-name-input"].Value)
		fmt.Println("last:", ic.View.State.Values["last-name-block"]["last-name-input"].Value)
	} else {
		log.Info().Str("body", string(body)).Msg("unexpected interaction")
	}
}

func generateReflectionModal() slack.ModalViewRequest {
	// Create a ModalViewRequest with a header and two inputs
	titleText := slack.NewTextBlockObject(slack.PlainTextType, "Good Day Tracker", false, false)
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Close", false, false)
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Submit", false, false)

	headerText := slack.NewTextBlockObject(slack.MarkdownType, "Time to think about how the day went. Pick the answers that are closest to how you felt today went, and we'll review for patterns at the end of the week.", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	var qualitySelect *slack.SelectBlockElement
	{
		ter := slack.NewOptionBlockObject("0-terrible", slack.NewTextBlockObject(slack.PlainTextType, "Terrible", false, false), nil)
		bad := slack.NewOptionBlockObject("1-bad", slack.NewTextBlockObject(slack.PlainTextType, "Bad", false, false), nil)
		ok := slack.NewOptionBlockObject("2-ok", slack.NewTextBlockObject(slack.PlainTextType, "OK", false, false), nil)
		good := slack.NewOptionBlockObject("3-good", slack.NewTextBlockObject(slack.PlainTextType, "Good", false, false), nil)
		awe := slack.NewOptionBlockObject("4-awesome", slack.NewTextBlockObject(slack.PlainTextType, "Awesome", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "Pick the closest", false, false)
		qualitySelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "", ter, bad, ok, good, awe)
	}

	var amountOfDaySelect *slack.SelectBlockElement
	{
		none := slack.NewOptionBlockObject("0-none", slack.NewTextBlockObject(slack.PlainTextType, "None of the day", false, false), nil)
		little := slack.NewOptionBlockObject("1-little", slack.NewTextBlockObject(slack.PlainTextType, "A little of the day", false, false), nil)
		some := slack.NewOptionBlockObject("2-some", slack.NewTextBlockObject(slack.PlainTextType, "Some of the day", false, false), nil)
		much := slack.NewOptionBlockObject("3-much", slack.NewTextBlockObject(slack.PlainTextType, "Much of the day", false, false), nil)
		most := slack.NewOptionBlockObject("4-most", slack.NewTextBlockObject(slack.PlainTextType, "Most or all of the day", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "How much of the day", false, false)
		amountOfDaySelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "", none, little, some, much, most)
	}

	var feelingSelect *slack.SelectBlockElement
	{
		tense := slack.NewOptionBlockObject("0-tense", slack.NewTextBlockObject(slack.PlainTextType, "Tense or nervous", false, false), nil)
		stress := slack.NewOptionBlockObject("1-stress", slack.NewTextBlockObject(slack.PlainTextType, "Stressed or upset", false, false), nil)
		sad := slack.NewOptionBlockObject("2-sad", slack.NewTextBlockObject(slack.PlainTextType, "Sad or depressed", false, false), nil)
		bored := slack.NewOptionBlockObject("3-bored", slack.NewTextBlockObject(slack.PlainTextType, "Bored", false, false), nil)
		calm := slack.NewOptionBlockObject("4-calm", slack.NewTextBlockObject(slack.PlainTextType, "Calm or relaxed", false, false), nil)
		serene := slack.NewOptionBlockObject("5-serene", slack.NewTextBlockObject(slack.PlainTextType, "Serene or content", false, false), nil)
		happy := slack.NewOptionBlockObject("6-happy", slack.NewTextBlockObject(slack.PlainTextType, "Happy or elated", false, false), nil)
		excited := slack.NewOptionBlockObject("7-excited", slack.NewTextBlockObject(slack.PlainTextType, "Excited or alert", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "Pick the closest", false, false)
		feelingSelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "", tense, stress, sad, bored, calm, serene, happy, excited)
	}

	var numberSelect *slack.SelectBlockElement
	{
		none := slack.NewOptionBlockObject("0-none", slack.NewTextBlockObject(slack.PlainTextType, "0", false, false), nil)
		one := slack.NewOptionBlockObject("1-one", slack.NewTextBlockObject(slack.PlainTextType, "1", false, false), nil)
		two := slack.NewOptionBlockObject("2-two", slack.NewTextBlockObject(slack.PlainTextType, "2", false, false), nil)
		few := slack.NewOptionBlockObject("3-few", slack.NewTextBlockObject(slack.PlainTextType, "3-4", false, false), nil)
		many := slack.NewOptionBlockObject("4-many", slack.NewTextBlockObject(slack.PlainTextType, "5 or more", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "How many", false, false)
		numberSelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "", none, one, two, few, many)
	}
	var timeSelect *slack.SelectBlockElement
	{
		morning := slack.NewOptionBlockObject("0-morning", slack.NewTextBlockObject(slack.PlainTextType, "In the morning (9:00 – 11:00)", false, false), nil)
		midday := slack.NewOptionBlockObject("1-midday", slack.NewTextBlockObject(slack.PlainTextType, "Mid-day (11:00 – 13:00)", false, false), nil)
		earlyAft := slack.NewOptionBlockObject("2-earlyAft", slack.NewTextBlockObject(slack.PlainTextType, "In the early afternoon (13:00 – 15:00)", false, false), nil)
		lateAft := slack.NewOptionBlockObject("3-lateAft", slack.NewTextBlockObject(slack.PlainTextType, "In the late afternoon (15:00 – 17:00)", false, false), nil)
		nonwork := slack.NewOptionBlockObject("4-nonwork", slack.NewTextBlockObject(slack.PlainTextType, "Outside typical work hours", false, false), nil)
		equally := slack.NewOptionBlockObject("5-equally", slack.NewTextBlockObject(slack.PlainTextType, "Equally throughout the day", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "Which part of the day", false, false)
		timeSelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "", morning, midday, earlyAft, lateAft, nonwork, equally)
	}

	// Questions
	q1Text := slack.NewTextBlockObject(slack.PlainTextType, "How was your work day?", false, false)
	q1 := slack.NewSectionBlock(q1Text, nil, slack.NewAccessory(qualitySelect))

	q2Text := slack.NewTextBlockObject(slack.PlainTextType, "I worked with other people", false, false)
	q2 := slack.NewSectionBlock(q2Text, nil, slack.NewAccessory(amountOfDaySelect))

	q3Text := slack.NewTextBlockObject(slack.PlainTextType, "I helped other people", false, false)
	q3 := slack.NewSectionBlock(q3Text, nil, slack.NewAccessory(amountOfDaySelect))

	q4Text := slack.NewTextBlockObject(slack.PlainTextType, "My work was interrupted", false, false)
	q4 := slack.NewSectionBlock(q4Text, nil, slack.NewAccessory(amountOfDaySelect))

	q5Text := slack.NewTextBlockObject(slack.PlainTextType, "I made progress toward my goals", false, false)
	q5 := slack.NewSectionBlock(q5Text, nil, slack.NewAccessory(amountOfDaySelect))

	q6Text := slack.NewTextBlockObject(slack.PlainTextType, "I did high-quality work", false, false)
	q6 := slack.NewSectionBlock(q6Text, nil, slack.NewAccessory(amountOfDaySelect))

	q7Text := slack.NewTextBlockObject(slack.PlainTextType, "I did a lot of work", false, false)
	q7 := slack.NewSectionBlock(q7Text, nil, slack.NewAccessory(amountOfDaySelect))

	q8Text := slack.NewTextBlockObject(slack.PlainTextType, "Which best describes how you feel about your work day?", false, false)
	q8 := slack.NewSectionBlock(q8Text, nil, slack.NewAccessory(feelingSelect))

	q9Text := slack.NewTextBlockObject(slack.PlainTextType, "My day was stressful", false, false)
	q9 := slack.NewSectionBlock(q9Text, nil, slack.NewAccessory(amountOfDaySelect))

	q10Text := slack.NewTextBlockObject(slack.PlainTextType, "I took breaks today", false, false)
	q10 := slack.NewSectionBlock(q10Text, nil, slack.NewAccessory(amountOfDaySelect))

	q11Text := slack.NewTextBlockObject(slack.PlainTextType, "How many meetings did you have today?", false, false)
	q11 := slack.NewSectionBlock(q11Text, nil, slack.NewAccessory(numberSelect))

	q12Text := slack.NewTextBlockObject(slack.PlainTextType, "Today, I felt most productive:", false, false)
	q12 := slack.NewSectionBlock(q12Text, nil, slack.NewAccessory(timeSelect))

	q13Text := slack.NewTextBlockObject(slack.PlainTextType, "Today, I felt least productive:", false, false)
	q13 := slack.NewSectionBlock(q13Text, nil, slack.NewAccessory(timeSelect))

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			q1, q2, q3, q4, q5, q6, q7, q8, q9, q10, q11, q12, q13,
		},
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = slack.ViewType("modal")
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	modalRequest.CallbackID = reflectionModalCallbackID
	return modalRequest
}

func startReflectionDialog(triggerID string) error {
	v := generateReflectionModal()
	_, err := sapi.OpenView(triggerID, v)
	if err != nil {
		return fmt.Errorf("error opening reflection modal: %w", err)
	}
	return nil
}

func handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ev, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Debug().Err(err).Str("body", string(body)).Msg("unable to parse event")
	}

	switch ev.Type {
	case slackevents.URLVerification:
		if uv, ok := ev.Data.(*slackevents.EventsAPIURLVerificationEvent); ok {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(uv.Challenge))
		} else {
			log.Debug().Str("body", string(body)).Msg("wrong type cast for URLVerification")
			fmt.Printf("data: %+v\n", ev.Data)
		}

	case slackevents.CallbackEvent:
		handleInnerEvent(r.Context(), w, ev.InnerEvent)

	default:
		fmt.Printf("ev: %+v\n", ev)
		log.Debug().Err(err).Str("evtype", ev.Type).Msg("unknown event type")

	}
}

func handleInnerEvent(ctx context.Context, w http.ResponseWriter, iev slackevents.EventsAPIInnerEvent) {
	switch ev := iev.Data.(type) {
	case *slackevents.AppHomeOpenedEvent:
		bb, err := renderHomeView(ctx, ev.User)
		if err != nil {
			log.Debug().Err(err).Str("user", ev.User).Msg("error rendering home view")
			w.WriteHeader(http.StatusInternalServerError)
		}

		v := slack.HomeTabViewRequest{
			Type:   slack.VTHomeTab,
			Blocks: bb,
		}
		r, err := sapi.PublishViewContext(ctx, ev.User, v, ev.View.Hash)
		if err != nil {
			log.Debug().Err(err).Str("user", ev.User).Msg("error publishing home view")
		} else {
			fmt.Println("published!")
			fmt.Printf("%+v\n", r)
		}

	default:
		fmt.Printf("unknown inner event type: %+v", ev)
	}
}

func renderHomeView(ctx context.Context, uid string) (slack.Blocks, error) {
	var bb slack.Blocks

	bb.BlockSet = append(bb.BlockSet, slack.NewSectionBlock(nil, []*slack.TextBlockObject{slack.NewTextBlockObject(slack.MarkdownType, fmt.Sprintf("hello %s!", uid), false, false)}, nil))
	bb.BlockSet = append(bb.BlockSet, slack.NewActionBlock("home-start-reflection-action-block", slack.NewButtonBlockElement(homeButtonStartReflection, "start-today-btn", slack.NewTextBlockObject(slack.PlainTextType, "Reflect on Today", false, false))))

	return bb, nil
}

func viewHash(uid string) string {
	return fmt.Sprintf("%x", xxh3.HashString(fmt.Sprintf("%d:%s", time.Now().UnixNano(), uid)))
}

func printBody(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	fmt.Println(string(b), err)
}

const (
	homeButtonStartReflection = "start-reflection-action"
	reflectionModalCallbackID = "reflection-modal-callback-id"
)
