package zulip

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// A helper function to handle HTTP requests to the Zulip server specified in the context
func makeZulipRequest(context *Context, params url.Values, url string, method string) (resp *http.Response, err error) {
	var transport *http.Transport
	if context.Secure == false {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	} else {
		transport = &http.Transport{}
	}

	client := &http.Client{
		Transport: transport,
	}

	req, err := http.NewRequest(method, context.APIBase+"/"+url, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(context.Email, context.APIKey)
	req.Header.Add("Content-type", "application/x-www-form-urlencoded")
	return client.Do(req)
}

// A helper function to handle sending a private or stream message
func sendMessage(context *Context,
	msgType MessageType,
	messageContent string,
	messageTo []string,
	messageSubject string) (msgid uint64, err error) {

	params := url.Values{}

	switch msgType {
	case PrivateMessage:
		params.Add("type", "private")
	case StreamMessage:
		params.Add("type", "stream")
		if messageSubject == "" {
			return 0, fmt.Errorf("Subject (topic) is required when sending a stream message")
		}
	default:
		log.Panic("Implementation failure! Valid MessageType not handled by zulipai.SendMessage().")
	}

	params.Add("subject", messageSubject)
	params.Add("to", strings.Join(messageTo, ","))
	params.Add("content", messageContent)

	resp, err := makeZulipRequest(context, params, "messages", "POST")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body := json.NewDecoder(resp.Body)

	var ret zulipSendMessageReturn
	err = body.Decode(&ret)
	if err != nil {
		return 0, err
	}

	if ret.Result != zulipSuccessResult {
		return 0, fmt.Errorf("API call returned %s: %s", ret.Result, ret.Message)
	}

	return ret.ID, nil
}

// SendPrivateMessage sends a private message using the Context.
// messageTo is an array of email addresses to which to send the message.
// messageSubject may be blank.
func SendPrivateMessage(context *Context,
	messageContent string,
	messageTo []string,
	messageSubject string) (msgid uint64, err error) {
	return sendMessage(context, PrivateMessage, messageContent, messageTo, messageSubject)
}

// SendStreamMessage sends a stream message using the Context.
// messageTo is the name of a stream to send to and messageSubject is the topic (which
// must not be blank).
func SendStreamMessage(context *Context,
	messageContent string,
	messageTo string,
	messageSubject string) (msgid uint64, err error) {
	return sendMessage(context, StreamMessage, messageContent, []string{messageTo}, messageSubject)
}

//
// func UpdateMessage() error {
//
// }
//
// func GetMessages() {
//
// }
//

// GetEvents retreives those events from the specified queue on the Zulip server
// which occurred after lastEventID. If doNotBlock is true then the server will
// return as soon as possible (a non-blocking reply). Otherwise the server will
// hold the request open (block) until a new event is available or a few minutes
// have passed (at which point the server will send a heartbeat event).
func GetEvents(context *Context,
	queueID uint64,
	lastEventID uint64,
	doNotBlock bool) (events []Event, err error) {
	params := url.Values{}
	params.Add("queue_id", strconv.FormatUint(queueID, 10))
	params.Add("last_event_id", strconv.FormatUint(lastEventID, 10))
	params.Add("dont_block", strconv.FormatBool(doNotBlock))

	resp, err := makeZulipRequest(context, params, "events", "GET")
	if err != nil {
		return []Event{}, err
	}
	defer resp.Body.Close()

	body := json.NewDecoder(resp.Body)

	var ret zulipEventsReturn
	err = body.Decode(&ret)
	if err != nil {
		return []Event{}, err
	}

	if ret.Result != zulipSuccessResult {
		return []Event{}, fmt.Errorf("API call returned %s: %s", ret.Result, ret.Message)
	}

	return ret.Events, nil
}

// Register creates a queue on the Zulip server that will be filled with events
// by the server, which can be retreived by a call to GetEvents(). Returns a
// queue ID and the initial value of lastEventID to be passed to GetEvents().
// eventTypes is a bit mask of the types of events to fill the queue with. If this
// is 0 then all events will be returned. If applyMarkdown is true then event
// text will be returned in HTML, otherwise markdown will be returned (as the user
// entered it).
func Register(context *Context,
	eventTypes EventType,
	applyMarkdown bool) (queueID uint64, lastEventID uint64, err error) {

	if eventTypes == 0 {
		eventTypes = MessageEvent | SubscriptionsEvent | RealmUserEvent | PointerEvent
	}

	params := url.Values{}
	params.Add("apply_markdown", strconv.FormatBool(applyMarkdown))

	jsonEventTypesArray := []string{}
	if eventTypes&MessageEvent > 0 {
		jsonEventTypesArray = append(jsonEventTypesArray, "message")
	}
	if eventTypes&SubscriptionsEvent > 0 {
		jsonEventTypesArray = append(jsonEventTypesArray, "subscriptions")
	}
	if eventTypes&RealmUserEvent > 0 {
		jsonEventTypesArray = append(jsonEventTypesArray, "realm_user")
	}
	if eventTypes&PointerEvent > 0 {
		jsonEventTypesArray = append(jsonEventTypesArray, "pointer")
	}
	jsonEventTypes, err := json.Marshal(jsonEventTypesArray)
	if err != nil {
		return 0, 0, err
	}
	params.Add("event_types", string(jsonEventTypes[:]))

	resp, err := makeZulipRequest(context, params, "register", "POST")
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body := json.NewDecoder(resp.Body)

	var ret zulipRegisterReturn
	err = body.Decode(&ret)
	if err != nil {
		return 0, 0, err
	}

	if ret.Result != zulipSuccessResult {
		return 0, 0, fmt.Errorf("API call returned %s: %s", ret.Result, ret.Message)
	}

	return ret.QueueID, ret.LastEventID, nil
}

// CanReachServer returns true iff the context represents a server which can be reached successfully
// Uses the generate_204 Zuplip API endpoint
func CanReachServer(context *Context) error {
	resp, err := makeZulipRequest(context, url.Values{}, "generate_204", "GET")
	if err != nil {
		return err
	}
	if resp.StatusCode != 204 {
		return fmt.Errorf("%s", resp.Status)
	}
	return nil
}

//
// func Export() {
//
// }
//
// func Deregister(context *Context, queue uint64) (err error) {
//
// }
//
// func GetProfile() {
//
// }
//
// func GetStreams() {
//
// }
//
// func GetMembers() {
//
// }
//
// func ListSubscriptions() {
//
// }
//
// func AddSubscriptions() {
//
// }
//
// func RemoveSubscriptions() {
//
// }
//
// func GetSubscribers() {
//
// }
//
// func RenderMessage() {
//
// }
//
// func CreateUser() {
//
// }

// These come from https://github.com/zulip/zulip/blob/master/api/zulip/__init__.py
//
// Client._register('send_message', url='messages', make_request=(lambda request: request))
// Client._register('update_message', method='PATCH', url='messages', make_request=(lambda request: request))
// Client._register('get_messages', method='GET', url='messages/latest', longpolling=True)
// Client._register('get_events', url='events', method='GET', longpolling=True, make_request=(lambda **kwargs: kwargs))
// Client._register('register', make_request=_mk_events)
// Client._register('export', method='GET', url='export')
// Client._register('deregister', url="events", method="DELETE", make_request=_mk_deregister)
// Client._register('get_profile', method='GET', url='users/me')
// Client._register('get_streams', method='GET', url='streams', make_request=_kwargs_to_dict)
// Client._register('get_members', method='GET', url='users')
// Client._register('list_subscriptions', method='GET', url='users/me/subscriptions')
// Client._register('add_subscriptions', url='users/me/subscriptions', make_request=_mk_subs)
// Client._register('remove_subscriptions', method='PATCH', url='users/me/subscriptions', make_request=_mk_rm_subs)
// Client._register('get_subscribers', method='GET',
//                  computed_url=lambda request: 'streams/%s/members' % (urllib.parse.quote(request['stream'], safe=''),),
//                  make_request=_kwargs_to_dict)
// Client._register('render_message', method='GET', url='messages/render')
// Client._register('create_user', method='POST', url='users')

// ZULIP VIEWS
// URL base is https://zulip.com/api/v1/
// Messages are JSON
// Authentication is HTTP basic with
//      username = email address
//      password = api key (32 chars)
//
//
// These come from zulip server's urls.py:
//
// # JSON format views used by the redesigned API, accept basic auth username:password.
// v1_api_and_json_patterns = patterns('zerver.views',
//     url(r'^export$', 'rest_dispatch',
//             {'GET':  'export'}),
//     url(r'^users/me$', 'rest_dispatch',
//             {'GET': 'get_profile_backend'}),
//     url(r'^users/me/pointer$', 'rest_dispatch',
//             {'GET': 'get_pointer_backend',
//              'PUT': 'update_pointer_backend'}),
//     url(r'^realm$', 'rest_dispatch',
//             {'PATCH': 'update_realm'}),
//     url(r'^users/me/presence$', 'rest_dispatch',
//             {'POST': 'update_active_status_backend'}),
//     # Endpoint used by iOS devices to register their
//     # unique APNS device token
//     url(r'^users/me/apns_device_token$', 'rest_dispatch',
//         {'POST'  : 'add_apns_device_token',
//          'DELETE': 'remove_apns_device_token'}),
//     url(r'^users/me/android_gcm_reg_id$', 'rest_dispatch',
//         {'POST': 'add_android_reg_id',
//          'DELETE': 'remove_android_reg_id'}),
//     url(r'^register$', 'rest_dispatch',
//             {'POST': 'api_events_register'}),
//
//     # Returns a 204, used by desktop app to verify connectivity status
//     url(r'generate_204$', 'generate_204'),
//
// ) + patterns('zerver.views.users',
//     url(r'^users$', 'rest_dispatch',
//         {'GET': 'get_members_backend',
//          'POST': 'create_user_backend'}),
//     url(r'^users/(?P<email>.*)/reactivate$', 'rest_dispatch',
//         {'POST': 'reactivate_user_backend'}),
//     url(r'^users/(?P<email>[^/]*)$', 'rest_dispatch',
//         {'PATCH': 'update_user_backend',
//          'DELETE': 'deactivate_user_backend'}),
//     url(r'^bots$', 'rest_dispatch',
//         {'GET': 'get_bots_backend',
//          'POST': 'add_bot_backend'}),
//     url(r'^bots/(?P<email>.*)/api_key/regenerate$', 'rest_dispatch',
//         {'POST': 'regenerate_bot_api_key'}),
//     url(r'^bots/(?P<email>.*)$', 'rest_dispatch',
//         {'PATCH': 'patch_bot_backend',
//          'DELETE': 'deactivate_bot_backend'}),
//
// ) + patterns('zerver.views.messages',
//     # GET returns messages, possibly filtered, POST sends a message
//     url(r'^messages$', 'rest_dispatch',
//             {'GET':  'get_old_messages_backend',
//              'PATCH': 'update_message_backend',
//              'POST': 'send_message_backend'}),
//     url(r'^messages/render$', 'rest_dispatch',
//             {'GET':  'render_message_backend'}),
//     url(r'^messages/flags$', 'rest_dispatch',
//             {'POST':  'update_message_flags'}),
//
// ) + patterns('zerver.views.alert_words',
//     url(r'^users/me/alert_words$', 'rest_dispatch',
//         {'GET': 'list_alert_words',
//          'POST': 'set_alert_words',
//          'PUT': 'add_alert_words',
//          'DELETE': 'remove_alert_words'}),
//
// ) + patterns('zerver.views.user_settings',
//     url(r'^users/me/api_key/regenerate$', 'rest_dispatch',
//         {'POST': 'regenerate_api_key'}),
//     url(r'^users/me/enter-sends$', 'rest_dispatch',
//         {'POST': 'change_enter_sends'}),
//
// ) + patterns('zerver.views.streams',
//     url(r'^streams$', 'rest_dispatch',
//         {'GET':  'get_streams_backend'}),
//     # GET returns "stream info" (undefined currently?), HEAD returns whether stream exists (200 or 404)
//     url(r'^streams/(?P<stream_name>.*)/members$', 'rest_dispatch',
//         {'GET': 'get_subscribers_backend'}),
//     url(r'^streams/(?P<stream_name>.*)$', 'rest_dispatch',
//         {'HEAD': 'stream_exists_backend',
//          'GET': 'stream_exists_backend',
//          'PATCH': 'update_stream_backend',
//          'DELETE': 'deactivate_stream_backend'}),
//     url(r'^default_streams$', 'rest_dispatch',
//         {'PATCH': 'add_default_stream',
//          'DELETE': 'remove_default_stream'}),
//     # GET lists your streams, POST bulk adds, PATCH bulk modifies/removes
//     url(r'^users/me/subscriptions$', 'rest_dispatch',
//         {'GET': 'list_subscriptions_backend',
//          'POST': 'add_subscriptions_backend',
//          'PATCH': 'update_subscriptions_backend'}),
//
// ) + patterns('zerver.tornadoviews',
//     url(r'^events$', 'rest_dispatch',
//         {'GET': 'get_events_backend',
//          'DELETE': 'cleanup_event_queue'}),
// )
// if not settings.VOYAGER:
//     v1_api_and_json_patterns += patterns('',
//         # Still scoped to api/v1/, but under a different project
//         url(r'^deployments/', include('zilencer.urls.api')),
//     )
//
//     urlpatterns += patterns('',
//         url(r'^', include('zilencer.urls.pages')),
//     )
//
//     urlpatterns += patterns('',
//         url(r'^', include('analytics.urls')),
//     )
//
//     urlpatterns += patterns('',
//         url(r'^', include('corporate.urls')),
//     )
//
//
// urlpatterns += patterns('zerver.tornadoviews',
//     # Tornado views
//     url(r'^json/get_events$',               'json_get_events'),
//     # Used internally for communication between Django and Tornado processes
//     url(r'^notify_tornado$',                'notify'),
// )
//
// # Include the dual-use patterns twice
// urlpatterns += patterns('',
//     url(r'^api/v1/', include(v1_api_and_json_patterns)),
//     url(r'^json/', include(v1_api_and_json_patterns)),
// )
