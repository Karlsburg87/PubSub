//startSSE kick starts inflow of message stream from server
function startSSE(){
  const source = new EventSource("http://localhost:8080/sse")
  
  source.onmessage = (event) => {
    console.log("OnMessage Called:")
    console.log(event)
    console.log(JSON.parse(event.data))
  }
}