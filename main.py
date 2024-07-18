import pandas as pd

xls = pd.ExcelFile(r"park_4_night.xls") # use r before absolute file path

sheetX = xls.parse(0) #2 is the sheet number+1 thus if the file has only 1 sheet write 0 in paranthesis
# extract relevant columns
col_names = [col_name for i, col_name in zip(range(27), sheetX.columns)]
col_names.pop(0)
col_names.pop(-1)
col_names.pop(-1)

# custom melt i guess
data = {'DATUM': [], 'DRŽAVA': [], 'ODRASLI': [], 'OTROCI': [], 'KOLESARJI': []}

for i in range(len(sheetX['DATUM'])):
    k = 0
    for j in range(0, len(col_names), 4):
        if not str(sheetX[col_names[j]][i]) == 'nan':
            k += 1
            print(i, k)
    for j in range(k*4):
        data[col_names[j].split('.')[0]].append(sheetX[col_names[j]][i])
    for j in range(k):
        data['DATUM'].append(sheetX['DATUM'][i])
# to excel
new_df = pd.DataFrame(data)
new_df.to_excel('transformed.xlsx')

