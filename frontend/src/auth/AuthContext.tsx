import { createContext,useCallback,useContext,useEffect,useMemo,useState,type ReactNode } from "react";
import { controlApi,type AuthSession,type PublicUser } from "../api/control";
import { ApiError } from "../api/client";

type AuthContextValue={user:PublicUser|null;csrf:string;loading:boolean;mfaEnrollmentRequired:boolean;login:(username:string,password:string,totp?:string)=>Promise<AuthSession>;logout:()=>Promise<void>;logoutAll:()=>Promise<void>;refresh:()=>Promise<void>;hasPermission:(permission:string)=>boolean};
const AuthContext=createContext<AuthContextValue|null>(null);
export function AuthProvider({children}:{children:ReactNode}){
 const [session,setSession]=useState<AuthSession|null>(null);const [loading,setLoading]=useState(true);
 const refresh=useCallback(async()=>{try{setSession(await controlApi.me())}catch(e){if(e instanceof ApiError&&e.status===401)setSession(null);else throw e}finally{setLoading(false)}},[]);
 useEffect(()=>{void refresh()},[refresh]);
 const login=useCallback(async(username:string,password:string,totp?:string)=>{const next=await controlApi.login(username,password,totp);setSession(next);return next},[]);
 const logout=useCallback(async()=>{if(session?.csrfToken){try{await controlApi.logout(session.csrfToken)}finally{setSession(null)}}else setSession(null)},[session]);
 const logoutAll=useCallback(async()=>{if(session?.csrfToken){try{await controlApi.logoutAll(session.csrfToken)}finally{setSession(null)}}else setSession(null)},[session]);
 const value=useMemo<AuthContextValue>(()=>({user:session?.user??null,csrf:session?.csrfToken??"",loading,mfaEnrollmentRequired:session?.mfaEnrollmentRequired??false,login,logout,logoutAll,refresh,hasPermission:(p)=>session?.user.permissions.includes(p)??false}),[session,loading,login,logout,logoutAll,refresh]);
 return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
export function useAuth(){const v=useContext(AuthContext);if(!v)throw new Error("useAuth requires AuthProvider");return v}
