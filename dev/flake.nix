{
  description = "Inputs only necessary for developing devagent.";
  inputs = {
    devshell.url = "github:numtide/devshell";
    generate-go-sri.url = "github:antifuchs/generate-go-sri";
    tsnsrv.url = "github:boinkor-net/tsnsrv";
  };
  outputs = {...}: {};
}
