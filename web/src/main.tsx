import { render } from "preact";
import { App } from "./App";
import "./index.css";
import "@xterm/xterm/css/xterm.css";

const root = document.getElementById("root")!;
render(<App />, root);
