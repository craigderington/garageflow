"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import { AuthContext, type AuthContextType } from "@/hooks/useAuth";
import { api } from "@/lib/api";
import type { User } from "@/lib/types";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.get<{ user_id: string; shop_id: string; role: string }>("/auth/me")
      .then((data) =>
        setUser({
          id: data.user_id,
          shop_id: data.shop_id,
          role: data.role as User["role"],
          email: "",
          name: "",
        })
      )
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    await api.post("/auth/login", { email, password });
    const data = await api.get<{ user_id: string; shop_id: string; role: string }>("/auth/me");
    setUser({
      id: data.user_id,
      shop_id: data.shop_id,
      role: data.role as User["role"],
      email,
      name: "",
    });
  }, []);

  // The API sets the session cookie on /auth/verify, so this mirrors login():
  // exchange the code, then read back who we are.
  const verifyCode = useCallback(async (code: string) => {
    await api.post("/auth/verify", { code });
    const data = await api.get<{ user_id: string; shop_id: string; role: string }>("/auth/me");
    setUser({
      id: data.user_id,
      shop_id: data.shop_id,
      role: data.role as User["role"],
      email: "",
      name: "",
    });
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.post("/auth/logout");
    } catch {
      // ignore
    } finally {
      setUser(null);
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
    }
  }, []);

  const value: AuthContextType = { user, loading, login, verifyCode, logout };
  return <AuthContext value={value}>{children}</AuthContext>;
}
