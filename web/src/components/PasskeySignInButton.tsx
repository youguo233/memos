import { FingerprintIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/contexts/AuthContext";
import useNavigateTo from "@/hooks/useNavigateTo";
import { passkeySupported, signInWithPasskey } from "@/lib/passkey";
import { ROUTES } from "@/router/routes";
import { useTranslate } from "@/utils/i18n";

function PasskeySignInButton({ redirectPath }: { redirectPath?: string }) {
  const t = useTranslate();
  const navigateTo = useNavigateTo();
  const { initialize } = useAuth();
  const [loading, setLoading] = useState(false);

  if (!passkeySupported()) return null;

  const handleClick = async () => {
    setLoading(true);
    try {
      await signInWithPasskey();
      await initialize();
      navigateTo(redirectPath || ROUTES.HOME, { replace: true });
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : String(error));
    }
    setLoading(false);
  };

  return (
    <Button type="button" variant="outline" className="w-full" disabled={loading} onClick={handleClick}>
      <FingerprintIcon className="mr-1.5 size-4" />
      {t("auth.sign-in-with-passkey")}
    </Button>
  );
}

export default PasskeySignInButton;
