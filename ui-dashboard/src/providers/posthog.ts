const posthogConfig = {
    apiKey: import.meta.env.VITE_POSTHOG_KEY as string,
    options: {api_host: import.meta.env.VITE_POSTHOG_HOST as string}
};

export {posthogConfig};
