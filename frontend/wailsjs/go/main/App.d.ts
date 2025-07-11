// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT
import {data} from '../models';
import {models} from '../models';

export function AddCronTask(arg1:data.FollowedStock):Promise<any>;

export function AddGroup(arg1:data.Group):Promise<string>;

export function AddPrompt(arg1:models.Prompt):Promise<string>;

export function AddStockGroup(arg1:number,arg2:string):Promise<string>;

export function AnalyzeSentiment(arg1:string):Promise<data.SentimentResult>;

export function CheckSponsorCode(arg1:string):Promise<Record<string, any>>;

export function CheckUpdate():Promise<void>;

export function ClsCalendar():Promise<Array<any>>;

export function DelPrompt(arg1:number):Promise<string>;

export function EMDictCode(arg1:string):Promise<Array<any>>;

export function ExportConfig():Promise<string>;

export function Follow(arg1:string):Promise<string>;

export function FollowFund(arg1:string):Promise<string>;

export function GetAIResponseResult(arg1:string):Promise<models.AIResponseResult>;

export function GetConfig():Promise<data.Settings>;

export function GetFollowList(arg1:number):Promise<any>;

export function GetFollowedFund():Promise<Array<data.FollowedFund>>;

export function GetGroupList():Promise<Array<data.Group>>;

export function GetGroupStockList(arg1:number):Promise<Array<data.GroupStock>>;

export function GetHotStrategy():Promise<Record<string, any>>;

export function GetIndustryMoneyRankSina(arg1:string,arg2:string):Promise<Array<Record<string, any>>>;

export function GetIndustryRank(arg1:string,arg2:number):Promise<Array<any>>;

export function GetMoneyRankSina(arg1:string):Promise<Array<Record<string, any>>>;

export function GetPromptTemplates(arg1:string,arg2:string):Promise<any>;

export function GetSponsorInfo():Promise<Record<string, any>>;

export function GetStockCommonKLine(arg1:string,arg2:string,arg3:number):Promise<any>;

export function GetStockKLine(arg1:string,arg2:string,arg3:number):Promise<any>;

export function GetStockList(arg1:string):Promise<Array<data.StockBasic>>;

export function GetStockMinutePriceLineData(arg1:string,arg2:string):Promise<Record<string, any>>;

export function GetStockMoneyTrendByDay(arg1:string,arg2:number):Promise<Array<Record<string, any>>>;

export function GetTelegraphList(arg1:string):Promise<any>;

export function GetVersionInfo():Promise<models.VersionInfo>;

export function GetfundList(arg1:string):Promise<Array<data.FundBasic>>;

export function GlobalStockIndexes():Promise<Record<string, any>>;

export function Greet(arg1:string):Promise<data.StockInfo>;

export function HotEvent(arg1:number):Promise<any>;

export function HotStock(arg1:string):Promise<any>;

export function HotTopic(arg1:number):Promise<Array<any>>;

export function IndustryResearchReport(arg1:string):Promise<Array<any>>;

export function InvestCalendarTimeLine(arg1:string):Promise<Array<any>>;

export function LongTigerRank(arg1:string):Promise<any>;

export function NewChatStream(arg1:string,arg2:string,arg3:string,arg4:any,arg5:boolean):Promise<void>;

export function NewsPush(arg1:any):Promise<void>;

export function OpenURL(arg1:string):Promise<void>;

export function ReFleshTelegraphList(arg1:string):Promise<any>;

export function RemoveGroup(arg1:number):Promise<string>;

export function RemoveStockGroup(arg1:string,arg2:string,arg3:number):Promise<string>;

export function SaveAIResponseResult(arg1:string,arg2:string,arg3:string,arg4:string,arg5:string):Promise<void>;

export function SaveAsMarkdown(arg1:string,arg2:string):Promise<string>;

export function SaveImage(arg1:string,arg2:string):Promise<string>;

export function SaveWordFile(arg1:string,arg2:string):Promise<string>;

export function SearchStock(arg1:string):Promise<Record<string, any>>;

export function SendDingDingMessage(arg1:string,arg2:string):Promise<string>;

export function SendDingDingMessageByType(arg1:string,arg2:string,arg3:number):Promise<string>;

export function SetAlarmChangePercent(arg1:number,arg2:number,arg3:string):Promise<string>;

export function SetCostPriceAndVolume(arg1:string,arg2:number,arg3:number):Promise<string>;

export function SetStockAICron(arg1:string,arg2:string):Promise<void>;

export function SetStockSort(arg1:number,arg2:string):Promise<void>;

export function ShareAnalysis(arg1:string,arg2:string):Promise<string>;

export function StockNotice(arg1:string):Promise<Array<any>>;

export function StockResearchReport(arg1:string):Promise<Array<any>>;

export function SummaryStockNews(arg1:string,arg2:any,arg3:boolean):Promise<void>;

export function UnFollow(arg1:string):Promise<string>;

export function UnFollowFund(arg1:string):Promise<string>;

export function UpdateConfig(arg1:data.Settings):Promise<string>;
