import { createTamagui, TamaguiProvider, Text } from "@tamagui/core"
import { config } from "@tamagui/config/v3"
import React from "react"
import ReactDOM from "react-dom/client"
import App from "./App"

const tamaguiConfig = createTamagui(config)

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <TamaguiProvider config={tamaguiConfig}>
      <App />
    </TamaguiProvider>
  </React.StrictMode>,
)
