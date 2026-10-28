import { Editor } from "@usememos/editor";
import "@usememos/editor/style.css";
const el = document.getElementById("ed");
window.ed = new Editor({ element: el, content: "第一行", slashHint: true, autofocus: false });
