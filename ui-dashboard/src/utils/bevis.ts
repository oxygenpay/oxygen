type State = Record<string, string | boolean | number | undefined | null>;

interface ClassNameGenerator {
    (elementName?: string, state?: State): string;
    (state?: State): string;
}

const bevis = (blockName: string): ClassNameGenerator => {
    return (elementNameOrState?: string | State, state?: State): string => {
        let className = blockName;

        if (elementNameOrState) {
            if (typeof elementNameOrState === "string") {
                className += "__" + elementNameOrState;
            } else if (typeof elementNameOrState === "object") {
                state = elementNameOrState;
            }
        }

        if (state) {
            Object.keys(state).forEach((key) => {
                if (!state) {
                    return;
                }

                if (state[key] === true) {
                    className += " _" + key;
                } else if (state[key]) {
                    className += " _" + key + "_" + state[key];
                }
            });
        }

        return className;
    };
};

export default bevis;
export type {ClassNameGenerator, State};
