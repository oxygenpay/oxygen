import * as React from "react";
import {parseISO} from "date-fns";
import {format, utcToZonedTime} from "date-fns-tz";
import CollapseString from "src/components/collapse-string/collapse-string";

interface Props {
    time: string;
}

const TimeLabel: React.FC<Props> = (props: Props) => {
    const formatInTimeZone = (tz: string) => {
        return format(utcToZonedTime(parseISO(props.time), tz), "yyyy-MM-dd kk:mm", {timeZone: tz}) + " " + tz;
    };

    return (
        <CollapseString
            text={format(parseISO(props.time), "yyyy-MM-dd kk:mm")}
            popupText={formatInTimeZone("UTC")}
            collapseAt={20}
            withPopover
        />
    );
};

export default TimeLabel;
