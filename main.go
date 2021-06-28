package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var (
	sapi          *slack.Client
	signingSecret string
	db            *sql.DB
)

func main() {
	signingSecret = os.Getenv("SLACK_SIGNING_SECRET")
	sapi = slack.New(os.Getenv("SLACK_BOT_TOKEN"))
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = ":3000"
	}

	{
		dsn := os.Getenv("DATABASE_DSN")
		fmt.Println("connecting to db with dsn:", dsn)
		c, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Fatal().Err(err).Msg("error connecting to database")
		}
		db = c
		db.SetConnMaxLifetime(time.Minute * 3)
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(10)
	}

	http.HandleFunc("/", printBody)
	http.Handle("/event", verifySecret(http.HandlerFunc(handleEvent)))
	http.Handle("/interactive", verifySecret(http.HandlerFunc(handleInteractive)))
	http.Handle("/slash", verifySecret(http.HandlerFunc(handleSlash)))
	http.ListenAndServe(port, nil)
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

func handleSlash(w http.ResponseWriter, r *http.Request) {
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch s.Command {
	case "/reflect":
		params := &slack.Msg{Text: "Yay! Reflection time!"}
		b, err := json.Marshal(params)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)

		go startReflectionDialog(s.TriggerID)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
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
		/*
			for k, v := range ic.View.State.Values {
				for ik := range v {
					val := ic.View.State.Values[k][ik].SelectedOption.Value
					fmt.Println("k:", k, "ik:", ik, "iv.Value:", val)
				}
			}
		*/

		r := Reflection{
			TeamID:                ic.Team.ID,
			UserID:                ic.User.ID,
			Date:                  time.Now().Format("2006-01-02 15:04:05"),
			WorkDayQuality:        NumberPrefixedEnum(ic.View.State.Values["work_day_quality"]["quality-select"].SelectedOption.Value),
			WorkOtherPeopleAmount: NumberPrefixedEnum(ic.View.State.Values["work_other_people_amount"]["amount-select"].SelectedOption.Value),
			HelpOtherPeopleAmount: NumberPrefixedEnum(ic.View.State.Values["help_other_people_amount"]["amount-select"].SelectedOption.Value),
			InterruptedAmount:     NumberPrefixedEnum(ic.View.State.Values["interrupted_amount"]["amount-select"].SelectedOption.Value),
			ProgressGoalsAmount:   NumberPrefixedEnum(ic.View.State.Values["progress_goals_amount"]["amount-select"].SelectedOption.Value),
			QualityWorkAmount:     NumberPrefixedEnum(ic.View.State.Values["quality_work_amount"]["amount-select"].SelectedOption.Value),
			LotOfWorkAmount:       NumberPrefixedEnum(ic.View.State.Values["lot_of_work_amount"]["amount-select"].SelectedOption.Value),
			WorkDayFeeling:        NumberPrefixedEnum(ic.View.State.Values["work_day_feeling"]["feeling-select"].SelectedOption.Value),
			StressfulAmount:       NumberPrefixedEnum(ic.View.State.Values["stressful_amount"]["amount-select"].SelectedOption.Value),
			BreaksAmount:          NumberPrefixedEnum(ic.View.State.Values["breaks_amount"]["amount-select"].SelectedOption.Value),
			MeetingNumber:         NumberPrefixedEnum(ic.View.State.Values["meeting_number"]["number-select"].SelectedOption.Value),
			MostProductiveTime:    NumberPrefixedEnum(ic.View.State.Values["most_productive_time"]["time-select"].SelectedOption.Value),
			LeastProductiveTime:   NumberPrefixedEnum(ic.View.State.Values["least_productive_time"]["time-select"].SelectedOption.Value),
		}
		err := saveReflection(r)
		if err != nil {
			log.Error().Err(err).Msg("error saving reflection")
		}
		//} else {
		//log.Info().Str("body", string(body)).Msg("unexpected interaction")
	}
}

func saveReflection(r Reflection) error {
	_, err := db.Exec("INSERT INTO reflections SET team_id=?, user_id=?, date=?, work_day_quality=?, work_other_people_amount=?, help_other_people_amount=?, interrupted_amount=?, progress_goals_amount=?, quality_work_amount=?, lot_of_work_amount=?, work_day_feeling=?, stressful_amount=?, breaks_amount=?, meeting_number=?, most_productive_time=?, least_productive_time=?", r.TeamID, r.UserID, r.Date, r.WorkDayQuality, r.WorkOtherPeopleAmount, r.HelpOtherPeopleAmount, r.InterruptedAmount, r.ProgressGoalsAmount, r.QualityWorkAmount, r.LotOfWorkAmount, r.WorkDayFeeling, r.StressfulAmount, r.BreaksAmount, r.MeetingNumber, r.MostProductiveTime, r.LeastProductiveTime)
	if err != nil {
		return fmt.Errorf("error saving reflection: %w", err)
	}

	return nil
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
		qualitySelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "quality-select", ter, bad, ok, good, awe)
	}

	var amountOfDaySelect *slack.SelectBlockElement
	{
		none := slack.NewOptionBlockObject("0-none", slack.NewTextBlockObject(slack.PlainTextType, "None of the day", false, false), nil)
		little := slack.NewOptionBlockObject("1-little", slack.NewTextBlockObject(slack.PlainTextType, "A little of the day", false, false), nil)
		some := slack.NewOptionBlockObject("2-some", slack.NewTextBlockObject(slack.PlainTextType, "Some of the day", false, false), nil)
		much := slack.NewOptionBlockObject("3-much", slack.NewTextBlockObject(slack.PlainTextType, "Much of the day", false, false), nil)
		most := slack.NewOptionBlockObject("4-most", slack.NewTextBlockObject(slack.PlainTextType, "Most or all of the day", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "How much of the day", false, false)
		amountOfDaySelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "amount-select", none, little, some, much, most)
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
		feelingSelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "feeling-select", tense, stress, sad, bored, calm, serene, happy, excited)
	}

	var numberSelect *slack.SelectBlockElement
	{
		none := slack.NewOptionBlockObject("0-none", slack.NewTextBlockObject(slack.PlainTextType, "0", false, false), nil)
		one := slack.NewOptionBlockObject("1-one", slack.NewTextBlockObject(slack.PlainTextType, "1", false, false), nil)
		two := slack.NewOptionBlockObject("2-two", slack.NewTextBlockObject(slack.PlainTextType, "2", false, false), nil)
		few := slack.NewOptionBlockObject("3-few", slack.NewTextBlockObject(slack.PlainTextType, "3-4", false, false), nil)
		many := slack.NewOptionBlockObject("4-many", slack.NewTextBlockObject(slack.PlainTextType, "5 or more", false, false), nil)

		placeholder := slack.NewTextBlockObject(slack.PlainTextType, "How many", false, false)
		numberSelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "number-select", none, one, two, few, many)
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
		timeSelect = slack.NewOptionsSelectBlockElement("static_select", placeholder, "time-select", morning, midday, earlyAft, lateAft, nonwork, equally)
	}

	// Questions
	q1Text := slack.NewTextBlockObject(slack.PlainTextType, "How was your work day?", false, false)
	q1 := slack.NewInputBlock("work_day_quality", q1Text, qualitySelect)

	q2Text := slack.NewTextBlockObject(slack.PlainTextType, "I worked with other people", false, false)
	q2 := slack.NewInputBlock("work_other_people_amount", q2Text, amountOfDaySelect)

	q3Text := slack.NewTextBlockObject(slack.PlainTextType, "I helped other people", false, false)
	q3 := slack.NewInputBlock("help_other_people_amount", q3Text, amountOfDaySelect)

	q4Text := slack.NewTextBlockObject(slack.PlainTextType, "My work was interrupted", false, false)
	q4 := slack.NewInputBlock("interrupted_amount", q4Text, amountOfDaySelect)

	q5Text := slack.NewTextBlockObject(slack.PlainTextType, "I made progress toward my goals", false, false)
	q5 := slack.NewInputBlock("progress_goals_amount", q5Text, amountOfDaySelect)

	q6Text := slack.NewTextBlockObject(slack.PlainTextType, "I did high-quality work", false, false)
	q6 := slack.NewInputBlock("quality_work_amount", q6Text, amountOfDaySelect)

	q7Text := slack.NewTextBlockObject(slack.PlainTextType, "I did a lot of work", false, false)
	q7 := slack.NewInputBlock("lot_of_work_amount", q7Text, amountOfDaySelect)

	q8Text := slack.NewTextBlockObject(slack.PlainTextType, "Which best describes how you feel about your work day?", false, false)
	q8 := slack.NewInputBlock("work_day_feeling", q8Text, feelingSelect)

	q9Text := slack.NewTextBlockObject(slack.PlainTextType, "My day was stressful", false, false)
	q9 := slack.NewInputBlock("stressful_amount", q9Text, amountOfDaySelect)

	q10Text := slack.NewTextBlockObject(slack.PlainTextType, "I took breaks today", false, false)
	q10 := slack.NewInputBlock("breaks_amount", q10Text, amountOfDaySelect)

	q11Text := slack.NewTextBlockObject(slack.PlainTextType, "How many meetings did you have today?", false, false)
	q11 := slack.NewInputBlock("meeting_number", q11Text, numberSelect)

	q12Text := slack.NewTextBlockObject(slack.PlainTextType, "Today, I felt most productive:", false, false)
	q12 := slack.NewInputBlock("most_productive_time", q12Text, timeSelect)

	q13Text := slack.NewTextBlockObject(slack.PlainTextType, "Today, I felt least productive:", false, false)
	q13 := slack.NewInputBlock("least_productive_time", q13Text, timeSelect)

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
		_, err = sapi.PublishViewContext(ctx, ev.User, v, ev.View.Hash)
		if err != nil {
			log.Debug().Err(err).Str("user", ev.User).Msg("error publishing home view")
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

func printBody(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	fmt.Println(string(b), err)
}

const (
	homeButtonStartReflection = "start-reflection-action"
	reflectionModalCallbackID = "reflection-modal-callback-id"
)
