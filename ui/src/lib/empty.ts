import type {
  Action,
  Definition,
  File,
  Fn,
  Gate,
  General,
  GenericJSON,
  LiveFeed,
  Playable,
  Prefix,
  Queue,
  QueuePrompt,
  RandomFile,
  Recording,
  Service,
  SIPConfig,
  TTS,
} from "../generated/config";

// Factories for "blank" config objects. The editor uses these for new
// items and to ensure all fields are present (so React inputs are
// controlled).

export const emptyTTS = (): TTS => ({
  msg: "",
  voice: "",
  lang: "",
  engine: "",
  provider: "",
});

export const emptyFile = (): File => ({ src: "", block: false, clear: false });
export const emptyRandomFile = (): RandomFile => ({ folder: "" });
export const emptyRecording = (): Recording => ({ path: "" });
export const emptyPrefix = (): Prefix => ({
  file: emptyFile(),
  tts: emptyTTS(),
  wait: false,
  ignoreclear: false,
});
export const emptyService = (): Service => ({
  dst: "",
  tmpl: "",
  args: {},
  tts: emptyTTS(),
});
export const emptyGate = (): Gate => ({
  dst: "",
  accept: "",
  prompt: "",
  deny: "",
  deny_tmpl: "",
});
export const emptyLiveFeed = (): LiveFeed => ({ device: "", channel: 0 });
export const emptyGenericJSON = (): GenericJSON => ({
  url: "",
  method: "",
  body: "",
  headers: {},
  tmpl: "",
  timeout_seconds: 0,
  tts: emptyTTS(),
});

export const emptyAction = (): Action => ({
  num: 0,
  wait: false,
  clear: false,
  name: "",
  prefix: emptyPrefix(),
  pmsg: emptyPrefix(),
  dst: "",
  file: emptyFile(),
  randomfile: emptyRandomFile(),
  tts: emptyTTS(),
  srv: emptyService(),
  dispatcher: "",
  transfer: "",
  hangup: false,
  record: "",
  record_to: "",
  dtmf: "",
  livefeed: null,
  genericjson: emptyGenericJSON(),
});

export const emptyFn = (name = ""): Fn => ({
  name,
  recording: emptyRecording(),
  prefix: emptyPrefix(),
  gate: emptyGate(),
  clear_callstack: false,
  inputlength: 0,
  actions: [],
});

export const emptyPlayable = (): Playable => ({
  file: emptyFile(),
  tts: emptyTTS(),
  wait: false,
  clear: false,
});

export const emptyQueuePrompt = (): QueuePrompt => ({
  prompt: emptyPlayable(),
  empty: false,
  weight: 1,
});

export const emptyQueue = (name = ""): Queue => ({
  name,
  entrymsg: emptyPlayable(),
  minpos: 20,
  maxpos: 60,
  speed: 60,
  minprompt: 35,
  maxprompt: 120,
  currentpos: emptyTTS(),
  prompt: [],
  bgmusic: emptyFile(),
  end: emptyAction(),
});

export const emptyGeneral = (): General => ({
  entrypoint: "main",
  default_tts_voice: "",
  default_tts_lang: "",
  default_tts_engine: "",
  default_tts_provider: "",
});

export const emptySIP = (): SIPConfig => ({
  server: "",
  extension: "",
  username: "",
  domain: "",
  transport: "udp",
  local_port: 5060,
  max_concurrent_calls: 10,
  record_path: "files/recording",
  expiry_seconds: 300,
  external_ip: "",
  direct: false,
});

export const emptyDefinition = (): Definition => ({
  general: emptyGeneral(),
  sip: emptySIP(),
  fn: [],
  queue: [],
});
