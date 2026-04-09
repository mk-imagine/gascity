// Spike: Does Ink render correctly over SSH + docker exec -it?
//
// This is the critical unknown for the chat UI. Ink uses the same
// React terminal renderer as Claude Code's TUI, which hangs in Docker
// due to the PTY multiplexing bridge. If this simple Ink app works,
// the chat UI will too.
//
// Usage:
//   # Build image first (needs ink dependency)
//   # Then from local tmux pane:
//   ssh namurim docker exec -it <container> node /app/test-ink.mjs
//
// What to check:
//   1. Does it render without hanging?
//   2. Does keyboard input work (type text, press enter)?
//   3. Does the display update on each keystroke?

import React, { useState } from "react";
import { render, Box, Text, useInput, useApp } from "ink";
import TextInput from "ink-text-input";

function Chat() {
  const [messages, setMessages] = useState([]);
  const [input, setInput] = useState("");
  const { exit } = useApp();

  const handleSubmit = (value) => {
    if (value.trim() === "/quit") {
      exit();
      return;
    }
    if (value.trim()) {
      setMessages((prev) => [
        ...prev,
        { role: "user", text: value },
        { role: "assistant", text: `Echo: ${value}` },
      ]);
    }
    setInput("");
  };

  useInput((ch, key) => {
    if (key.ctrl && ch === "c") {
      exit();
    }
  });

  return React.createElement(
    Box,
    { flexDirection: "column", padding: 1 },
    // Header
    React.createElement(
      Box,
      { borderStyle: "single", paddingX: 1 },
      React.createElement(Text, { bold: true, color: "cyan" }, "Ink PTY Test — type messages, /quit to exit")
    ),
    // Messages
    React.createElement(
      Box,
      { flexDirection: "column", marginY: 1 },
      messages.length === 0
        ? React.createElement(Text, { dimColor: true }, "No messages yet. Type something and press Enter.")
        : messages.map((msg, i) =>
            React.createElement(
              Box,
              { key: i },
              React.createElement(
                Text,
                { color: msg.role === "user" ? "green" : "yellow" },
                `${msg.role === "user" ? "You" : "Bot"}: ${msg.text}`
              )
            )
          )
    ),
    // Input
    React.createElement(
      Box,
      null,
      React.createElement(Text, { color: "green" }, "> "),
      React.createElement(TextInput, {
        value: input,
        onChange: setInput,
        onSubmit: handleSubmit,
      })
    )
  );
}

render(React.createElement(Chat));
