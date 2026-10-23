import { Callout, Flex, IconButton } from "@radix-ui/themes";
import { Cross1Icon } from "@radix-ui/react-icons";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { TablerAlertTriangleFilled } from "@/components/Icones/Tabler";
import { TERMINAL_CALLOUT_CLASS_NAME } from "./terminalTypes";

interface TerminalNoticesProps {
  httpsCalloutOpen: boolean;
  onDismissHttpsCallout: () => void;
}

const TerminalNotices = ({
  httpsCalloutOpen,
  onDismissHttpsCallout,
}: TerminalNoticesProps) => {
  const { t } = useTranslation();

  return (
    <>
      <div className="pointer-events-none absolute inset-x-0 bottom-16 z-30 flex items-center justify-center px-3">
        <motion.div
          initial={{ opacity: 0, y: -20, scale: 0.95 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          transition={{ duration: 0.3, ease: "easeInOut" }}
          hidden={!httpsCalloutOpen}
          className="pointer-events-auto max-w-full"
        >
          <Callout.Root
            color="red"
            size="2"
            className={TERMINAL_CALLOUT_CLASS_NAME}
          >
            <Callout.Icon>
              <TablerAlertTriangleFilled className="text-red-400" />
            </Callout.Icon>
            <Callout.Text className="font-medium">
              <Flex align="center" justify="between" gap="3" className="min-w-0">
                <span className="min-w-0 whitespace-normal text-xs">{t("warn_https")}</span>
                <IconButton
                  variant="soft"
                  color="red"
                  size="1"
                  className="transition-colors hover:bg-red-500/20"
                  onClick={onDismissHttpsCallout}
                >
                  <Cross1Icon />
                </IconButton>
              </Flex>
            </Callout.Text>
          </Callout.Root>
        </motion.div>
      </div>
    </>
  );
};

export default TerminalNotices;
