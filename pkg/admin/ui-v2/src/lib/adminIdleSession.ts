import { useEffect, useRef } from 'react';

const ACTIVITY_EVENTS = ['mousedown', 'keydown', 'scroll', 'touchstart'] as const;

type UseAdminIdleLogoutOptions = {
  enabled: boolean;
  idleTimeoutSeconds: number;
  onIdle: () => void;
};

/** Calls onIdle after inactivity; must invoke server logout inside onIdle. */
export function useAdminIdleLogout({ enabled, idleTimeoutSeconds, onIdle }: UseAdminIdleLogoutOptions) {
  const deadlineRef = useRef<number>(0);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const onIdleRef = useRef(onIdle);
  onIdleRef.current = onIdle;

  useEffect(() => {
    if (!enabled || idleTimeoutSeconds <= 0) {
      return;
    }

    const bump = () => {
      deadlineRef.current = Date.now() + idleTimeoutSeconds * 1000;
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      timerRef.current = setTimeout(() => {
        if (Date.now() >= deadlineRef.current) {
          onIdleRef.current();
        }
      }, idleTimeoutSeconds * 1000);
    };

    bump();
    for (const ev of ACTIVITY_EVENTS) {
      window.addEventListener(ev, bump, { passive: true });
    }
    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
      for (const ev of ACTIVITY_EVENTS) {
        window.removeEventListener(ev, bump);
      }
    };
  }, [enabled, idleTimeoutSeconds]);
}
