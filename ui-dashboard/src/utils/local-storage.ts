function set(key: string, value: unknown): void {
    if (!isStorageAvailable()) {
        return;
    }

    try {
        window.localStorage.setItem(key, JSON.stringify(value));
    } catch (e) {
        if (e instanceof Error) {
            logError(e);
        }
    }
}

function get<T>(key: string): T | null {
    if (!isStorageAvailable()) {
        return null;
    }

    try {
        const value = window.localStorage.getItem(key);
        return value && JSON.parse(value);
    } catch (e) {
        if (e instanceof Error) {
            logError(e);
        }
        return null;
    }
}

function remove(key: string): void {
    if (!isStorageAvailable()) {
        return;
    }

    try {
        window.localStorage.removeItem(key);
    } catch (e) {
        if (e instanceof Error) {
            logError(e);
        }
    }
}

function isStorageAvailable(): boolean {
    try {
        return "localStorage" in window && Boolean(window.localStorage);
    } catch (e) {
        return false;
    }
}

const localStorage = {
    set,
    get,
    remove
};

function logError(e: Error): void {
    // eslint-disable-next-line no-console
    console.error(e);
}

export default localStorage;
