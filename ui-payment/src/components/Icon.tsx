import * as React from "react";
import LoadingCircleIcon from "./LoadingCircleIcon";

const cache: Record<string, React.ElementType | undefined> = {};

interface Props {
    name: string;
    dir?: string;
    className?: string;
}

const Icon: React.FC<Props> = (props) => {
    const [loaded, setLoaded] = React.useState<boolean>(Boolean(cache[props.name]));
    const loadIcon = async () => {
        const path = props.dir ? props.dir + "/" + props.name : props.name;
        cache[props.name] = (await import(`../assets/icons/${path}.svg`))?.ReactComponent;
        setLoaded(true);
    };

    React.useEffect(() => {
        if (!loaded) {
            try {
                loadIcon();
            } catch (e) {
                console.log(e instanceof Error ? e.message : "");
            }
        }
    }, [props.name]);

    const CurrentIcon = cache[props.name];
    return CurrentIcon && loaded ? (
        <CurrentIcon className={props.className} />
    ) : (
        <LoadingCircleIcon className={props.className} position="left" />
    );
};

export default Icon;
