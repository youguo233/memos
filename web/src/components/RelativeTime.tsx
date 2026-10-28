import type { Format } from "@github/relative-time-element";
import i18n from "@/i18n";
import { useUserTimeZone } from "@/lib/timezone";

interface RelativeTimeProps {
  date?: Date;
  format?: Format;
}

const RelativeTime = ({ date, format }: RelativeTimeProps) => {
  const timeZone = useUserTimeZone();
  return (
    <relative-time
      // Remounted on a zone change so the element re-renders its text.
      {...{ key: timeZone }}
      datetime={date?.toISOString()}
      lang={i18n.language}
      format={format}
      time-zone={timeZone}
      no-title=""
    ></relative-time>
  );
};

export default RelativeTime;
