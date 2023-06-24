import * as React from "react";
import LoadingCircleIcon from "./LoadingCircleIcon";

const cache: Record<string, React.ElementType | undefined> = {};

interface Props {
    name: string;
    className?: string;
}

const Icon: React.FC<Props> = ({name, className}) => {
    const [loaded, setLoaded] = React.useState<boolean>(Boolean(cache[name]));
    const loadIcon = async () => {
        cache[name] = (await import(`../assets/icons/${name}.svg`))?.ReactComponent;
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
    }, [name]);

    const CurrentIcon = cache[name];
    return CurrentIcon && loaded ? (
        <CurrentIcon className={className} />
    ) : (
        <LoadingCircleIcon className={className} position="left" />
    );
};

export default Icon;
