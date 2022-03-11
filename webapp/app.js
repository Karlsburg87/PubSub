//----------------------------------------------------GLOBALS
//Base URL for the SSE streaming service
const pubsubURL = window.location.protocol + "//" + window.location.host;
//List of topics use has selected to receive stream messages from
let streamTopics = new Map();
//The SSE connection
let SSE;
//---------------------------------------------EVENT LISTENERS

document.addEventListener('DOMContentLoaded', function () {
  //for the slideout navigation on mobile
  var sidenav = document.querySelectorAll('.sidenav');
  var instances = M.Sidenav.init(sidenav);
  //for the select dropdowns
  var elems = document.querySelectorAll('select');
  var instancesOfSelect = M.FormSelect.init(elems);
});
{
  let formQuery = document.getElementById("userQuery");
  formQuery.addEventListener('submit', userQuery, true);
}

//-------------------------------------------------------INITS

//Fetch and show topics from load
OutputTopics();

//=-----------------------------------------HANDLER FUNCTIONS

//adds event listeners to a sse function
function react(evt) {
  //handle incoming messages
  evt.addEventListener("message", function (event) {
    console.log(JSON.parse(event.data));
    AddStreamItem(JSON.parse(event.data));
  })
  //handle SSE errors
  evt.onerror = function (err) {
    console.error("EventSource failed:", err);
  };
}

//--------------------------------------------HELPER FUNCTIONS
//display topics as buttons on the left pane
function OutputTopics() {
  GetTopics()
    .then((topicList) => { AddTopicItems(topicList) })
    .catch(err => { console.error(err) })
}

//Used by outputTopics to fetch and display topics lists to page
async function GetTopics() {
  let response = await fetch(pubsubURL + "/topics/fetch?username=PubSubUI&password=public", {
    method: 'GET', // *GET, POST, PUT, DELETE, etc.
    mode: 'cors', // no-cors, *cors, same-origin
    cache: 'no-cache', // *default, no-cache, reload, force-cache, only-if-cached
    credentials: 'same-origin', // include, *same-origin, omit
    headers: {}
  });
  if (response.status !== 200 && response.status !== 201) {
    console.log("Fetching topics received " + response.status + " status resonse")
    return undefined;
  }
  try {
    data = await response.json();
  } catch (err) {
    console.error(err);
    return undefined;
  }
  console.log("Topics: ", data);
  return data;
}



//StartSSE opens a SSE connection and awaits responses
/*Response
//SSEResponse is the object sent to the client and identifies which topic the message came from
type SSEResponse struct {
  TopicName string  `json:"topic_name,omitempty"`
  Message   Message `json:"message"`
}
*/
function StartSSE() {
  queryString = "";
  for (const topicName of streamTopics.keys()) {
    queryString += `topic=${encodeURIComponent(topicName)}&`;
  }
  //console.log(queryString.slice(0, -1))
  return new EventSource(pubsubURL + "/sse?" + queryString.slice(0, -1));
}


//--------------------------------------------------HTML DOM ADDERS

//Run query
async function userQuery(evt) {
  evt.preventDefault();

  const output = document.getElementById("code-sample-output-landing-area");
  const reqBody = document.getElementById("reqBody");
  const reqString = document.getElementById("reqString");
  const form = document.getElementById("userQuery");

  let username = form.elements['username'].value;
  let password = form.elements['password'].value;
  let topic = form.elements['topic'].value;
  let message = form.elements['message'].value;
  let endpoint = form.elements['endpoint'].value;
  let messageID = form.elements['messageID'].value;
  let webhookURL = form.elements['webhook_url'].value;

  let payload = { username: username, password: password, topic: topic, message: message }
  let payloadAsQueryString = `?username=${username}&password=${password}`;
  if (topic) {
    payloadAsQueryString += `&topic=${topic}`;
  }
  if (message) { payloadAsQueryString += `&message=${message}`; }
  if (messageID) { payloadAsQueryString += `&message_id=${messageID}` }
  if (webhookURL) { payloadAsQueryString += `&webhook_url=${webhookURL}` }

  //get response from API
  let ops = {
    body: JSON.stringify(payload),
    method: 'POST',
    mode: 'same-origin'
  }
  let asJSON;
  let res = await fetch(endpoint, ops);
  const res2 = res.clone();
  try {
    asJSON = await res.json();
    asJSON = JSON.stringify(asJSON, null, "\t");
  } catch (e) {
    console.error(JSON.stringify(e));
    asJSON = await res2.text();
  }

  output.value = asJSON;
  M.textareaAutoResize(output);
  reqBody.value = JSON.stringify(payload, null, "\t");
  M.textareaAutoResize(reqBody);
  reqString.value = window.location.protocol + "//" + window.location.host + payloadAsQueryString;
  M.textareaAutoResize(reqString);


  M.updateTextFields();
  return false;
}

//Build all the topics buttons from a returned list of topics from ther server and add to page
function AddTopicItems(ListKeysResp) {
  const landingArea = document.getElementById("topic-list-area");
  //clear existing buttons
  clearChildren(landingArea);
  //input new buttons
  for (let item of ListKeysResp.topics) {
    if (landingArea.firstChild !== null) {
      landingArea.insertBefore(BuildTopicButton(item), landingArea.firstChild);
      continue;
    }
    landingArea.appendChild(BuildTopicButton(item));
  }
}

//Add SSE Stream item to page
function AddStreamItem(sseMsg) {
  const landingArea = document.getElementById("stream-dump")
  const widget = BuildStreamItem(sseMsg);
  landingArea.insertBefore(widget, landingArea.firstChild)
  //trim list if too many on page
  keepElementsBelowN(landingArea, 5);
}

//the selected topics name list above the sse stream
function AddTopicBadge(TopicNameString) {
  const landingArea = document.getElementById("selected-topics-land");
  landingArea.insertBefore(BuildSelectedTopicBadge(TopicNameString), landingArea.firstChild);
}

//Adds the item to the selected area of the page or removes if already there.
function ToggleActiveTopics(event) {
  event.preventDefault();
  let parent = event.currentTarget;
  let topicName = parent.dataset.topic

  const outArea = document.getElementById("selected-topics-land");
  //button icon
  let buttonIcon;
  for (let child of parent.children) {
    if (child.tagName === "A") {
      for (let grandchild of child.children) {
        if (grandchild.tagName === "I") {
          buttonIcon = grandchild;
          break;
        }
      }
    }
  }
  if (buttonIcon === undefined) {
    console.log("Could not locate button item");
  }

  if (streamTopics.has(topicName)) {
    streamTopics.delete(topicName);
    //get node with that topicname in dataset.topic
    for (let node of outArea.children) {
      if (node.dataset.topic === topicName) {
        outArea.removeChild(node);
        //visible unselected icon on button
        buttonIcon.innerHTML = "circle";
        break;
      }
    }
  } else {
    //no matches so add
    streamTopics.set(topicName, true);
    AddTopicBadge(topicName);
    //visible selected icon on button
    buttonIcon.innerHTML = "check_circle";
  }

  //update progress bar
  const progressBar = document.getElementById("doingStuff")
  progressBar.classList.add("hide")

  //--start the event stream anew
  //close existing eventStream if open
  if (SSE !== undefined) {
    try {
      SSE.close()
    } catch (e) {
      console.error(e)
    }
  }

  //Make new SSE connection if there is selected topics to stream. Else not needed.
  if (streamTopics.size > 0) {
    //progress bar to receiving stream
    progressBar.classList.remove("hide")
    //for the Server side event stream
    const evtSource = StartSSE();
    //handle incoming stream
    react(evtSource);
    //add to global
    SSE = evtSource
  }
}

//--------------------------------------------------------HTML DOM REMOVERS

//generic clearence of all children of the element
function clearChildren(parentHTMLNode) {
  while (parentHTMLNode.firstChild) {
    parentHTMLNode.removeChild(parentHTMLNode.lastChild)
  }
}

//ensures there are no more than N number of child elements of the parentNode
function keepElementsBelowN(parentNode, N) {
  console.log(parentNode, parentNode.children.length);
  if (parentNode.children.length > N) {
    while (parentNode.children.length > N) {
      parentNode.removeChild(parentNode.lastChild)
    }
  }
}

//---------------------------------------------------------HTML BUILDERS


function BuildStreamItem(SSEResponse = { topic_name: "test tn", message: { data: "test message" } }) {
  const dateOpts = { weekday: 'short', year: '2-digit', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', timeZoneName: 'short' };

  const row = document.createElement("div");
  row.classList.add("row");
  const col = document.createElement("div");
  col.classList.add("col", "s12", "m12", "l12");
  const card = document.createElement("div");
  card.classList.add("card", "horizontal");
  const cardContent = document.createElement("div");
  cardContent.classList.add("card-content");
  const title = document.createElement("p");
  const titleSpan = document.createElement("span");
  titleSpan.classList.add("card-title")
  const titleTxt = document.createTextNode(SSEResponse.message.data.substring(0, 10) + "...");
  const message = document.createElement("p");
  const messageTxt = document.createTextNode(SSEResponse.message.data);
  const cardAction = document.createElement("div");
  cardAction.classList.add("card-action");
  const topicName = document.createElement("span");
  topicName.classList.add("badge")
  const iconTopic = document.createElement("i");
  iconTopic.classList.add("tiny", "material-icons");
  const iconSelect = document.createTextNode("schema");
  const topicNameTxt = document.createTextNode(` ${SSEResponse.topic_name}`);
  const badge = document.createElement("blockquote");
  badge.classList.add("chip");
  const badgeMsgID = document.createElement("span");
  badgeMsgID.classList.add("badge", "new");
  badgeMsgID.setAttribute("data-badge-caption", "#")
  const badgeTxt = document.createTextNode(SSEResponse.message.id);
  const badgeCreatedTxt = document.createTextNode(
    `Created: ${new Date(SSEResponse.message.created).toLocaleDateString('en-GB', dateOpts)}`);


  badge.appendChild(badgeCreatedTxt);
  badgeMsgID.appendChild(badgeTxt);
  iconTopic.appendChild(iconSelect);
  topicName.appendChild(iconTopic);
  topicName.appendChild(topicNameTxt);
  cardAction.appendChild(topicName);
  cardAction.appendChild(badgeMsgID);
  message.appendChild(messageTxt);
  titleSpan.appendChild(titleTxt);
  title.appendChild(titleSpan);
  cardContent.appendChild(title);
  cardContent.appendChild(message);
  cardContent.appendChild(badge);
  card.appendChild(cardContent);
  card.appendChild(cardAction);
  col.appendChild(card);
  row.appendChild(col);
  return row;
}

//Build an individual topic button from string - returns the button Node handle
function BuildTopicButton(topicNameString) {
  const wrap = document.createElement("div");
  wrap.dataset.topic = topicNameString;
  wrap.onclick = ToggleActiveTopics;
  const link = document.createElement("a")
  link.setAttribute("href", "#")
  link.classList.add("waves-effect", "waves-light", "btn", "ps-marginb", "ps-marginr", "left");
  const icon = document.createElement("i");
  icon.classList.add("material-icons", "left")
  const iconSelect = document.createTextNode("circle")
  const txt = document.createTextNode(topicNameString);

  icon.appendChild(iconSelect);
  link.appendChild(icon);
  link.appendChild(txt);
  wrap.appendChild(link);
  /*
<div><a class="waves-effect waves-light btn ps-marginb ps-marginr left">button</a></div>
*/
  return wrap;
}

//Builds the badges that appear as selected topics badges above the stream
function BuildSelectedTopicBadge(topicString) {
  const badge = document.createElement("span");
  badge.classList.add("badge", "chip", "z-depth-1", "truncate", "ps-marginb");
  badge.dataset.topic = topicString;
  const badgeTxt = document.createTextNode(topicString);
  badge.appendChild(badgeTxt);
  return badge;
}