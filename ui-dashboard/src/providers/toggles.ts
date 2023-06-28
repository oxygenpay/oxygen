const toggles = new Map<string, string>([["feedback", import.meta.env.VITE_ENABLE_FEEDBACK]]);

function toggled(key: string): boolean {
    if (!toggles.has(key)) {
        return false;
    }

    return castBool(toggles.get(key) ?? "");
}

function castBool(v: string): boolean {
    return ["true", "t", "1", "yes"].includes(v.toLowerCase());
}

export {toggled};
