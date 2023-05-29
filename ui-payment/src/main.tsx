import React from "react";
import ReactDOM from "react-dom/client";
import {BrowserRouter} from "react-router-dom";
import posthog from "posthog-js";
import {posthogConfig} from "./providers/posthog";
import {PostHogProvider} from "posthog-js/react";
import App from "./App";

const strictMode = process.env.NODE_ENV === "production";

// https://posthog.com/docs/libraries/react
// eslint-disable-next-line @typescript-eslint/no-unsafe-argument
posthog.init(posthogConfig.apiKey, posthogConfig.options);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
    (strictMode && (
        <React.StrictMode>
            <PostHogProvider client={posthog}>
                <BrowserRouter>
                    <App />
                </BrowserRouter>
            </PostHogProvider>
        </React.StrictMode>
    )) || (
        <PostHogProvider client={posthog}>
            <BrowserRouter>
                <App />
            </BrowserRouter>
        </PostHogProvider>
    )
);
