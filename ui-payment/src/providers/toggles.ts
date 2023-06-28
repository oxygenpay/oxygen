const toggles = new Map<string, string>([["show_branding", import.meta.env.VITE_SHOW_BRANDING]]);

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
