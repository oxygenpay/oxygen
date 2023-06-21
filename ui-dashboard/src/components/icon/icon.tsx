import * as React from "react";
import {useMount} from "react-use";

const cache: Record<string, React.ElementType | undefined> = {};

interface Props {
    name: string;
    dir?: string;
    className?: string;
}

const Icon: React.FC<Props> = (props) => {
    const [loaded, setLoaded] = React.useState(Boolean(cache[props.name]));

    useMount(async () => {
        if (!loaded) {
            try {
                cache[props.name] = (
                    await import(`../../assets/icons/${props.dir ? props.dir + "/" : ""}${props.name}.svg`)
                )?.ReactComponent;
                setLoaded(true);
            } catch (e) {
                console.log(e instanceof Error ? e.message : "");
            }
        }
    });

    const CurrentIcon = cache[props.name];
    return CurrentIcon ? <CurrentIcon className={props.className} /> : null;
};

export default Icon;
