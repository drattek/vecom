-- =====================================================================
-- Vistas dyn.* de synapse-bridge sobre el lakehouse del Link to Fabric
-- (workspace DYN365-Fabric, lakehouse dataverse_vegusa_cds2_workspace_unq1552bea2e8d74bffb367119adb1a7).
--
-- Sustituyen a las vistas homónimas de Synapse serverless
-- (dyn365fosyn-ondemand / dataverse_vegusa_unq1552bea2e8d74bffb367119adb1a7).
-- Mismas columnas y alias, para que los repositorios Java no cambien su SQL.
--
-- Diferencias con la versión de Synapse (mismas reglas que Dyn365/BI/CONEXION_fabric.md):
--   - Tablas del Link en minúsculas y con dbo. explícito.
--   - La collation del lakehouse es Latin1_General_100_BIN2_UTF8 (distingue
--     mayúsculas): las comparaciones de texto van con UPPER() en ambos lados
--     (dataareaid llega como 'msb' y 'MSB').
--   - El Link conserva las filas borradas: ISNULL(IsDelete, 0) = 0.
--
-- Se despliega con infrastructure/fabric/deploy_views.py (orden: dependencias primero).
-- Cada bloque está separado por una línea "-- @view dyn.<Nombre>".
-- =====================================================================

-- @view dyn.ProductBrand
SELECT e.displayproductnumber AS DisplayProductNumber,
       e.searchname           AS SearchName,
       era.name               AS Name,
       erv.textvalue          AS TextValue,
       e.recid                AS RecId
FROM dbo.ecoresproduct e
         LEFT JOIN dbo.ecoresproductinstancevalue erpiv
                   ON e.recid = erpiv.product AND ISNULL(erpiv.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresattributevalue erav
                   ON erav.instancevalue = erpiv.recid AND ISNULL(erav.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresattribute era
                   ON era.recid = erav.attribute AND ISNULL(era.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresvalue ervv
                   ON erav.value = ervv.recid AND ISNULL(ervv.IsDelete, 0) = 0
         LEFT JOIN dbo.ecorestextvalue erv
                   ON erv.recid = ervv.recid AND ISNULL(erv.IsDelete, 0) = 0
WHERE UPPER(era.name) = 'MARCA'
  AND ISNULL(e.IsDelete, 0) = 0

-- @view dyn.FinDim
SELECT *
FROM (SELECT dimensionattributevalueset.recid           AS RECID,
             dimensionattribute.name                    AS DIM_NAME,
             dimensionattributevaluesetitem.displayvalue AS DIM_VALUE
      FROM dbo.dimensionattributevalueset
               JOIN dbo.dimensionattributevaluesetitem
                    ON dimensionattributevalueset.recid = dimensionattributevaluesetitem.dimensionattributevalueset
                        AND ISNULL(dimensionattributevaluesetitem.IsDelete, 0) = 0
               JOIN dbo.dimensionattributevalue
                    ON dimensionattributevaluesetitem.dimensionattributevalue = dimensionattributevalue.recid
                        AND ISNULL(dimensionattributevalue.IsDelete, 0) = 0
               JOIN dbo.dimensionattribute
                    ON dimensionattributevalue.dimensionattribute = dimensionattribute.recid
                        AND ISNULL(dimensionattribute.IsDelete, 0) = 0
      WHERE ISNULL(dimensionattributevalueset.IsDelete, 0) = 0) AS SourceTable
    PIVOT (
    MAX(DIM_VALUE)
    FOR DIM_NAME IN ([Departmento], [Marca], [Negocio], [Sucursal])
    ) AS PivotTable

-- @view dyn.ItemCost
SELECT itemid                                AS ItemId,
       SUM(qty)                              AS Qty,
       SUM(CASE
               WHEN statusreceipt = 2 THEN costamountphysical + costamountadjustment
               WHEN statusreceipt = 1 THEN costamountposted + costamountadjustment
               WHEN statusissue = 2 THEN costamountphysical + costamountadjustment
               WHEN statusissue = 1 THEN costamountposted + costamountadjustment
               ELSE 0 END)                   AS Value,
       CASE
           WHEN SUM(qty) > 0 THEN
               SUM(CASE
                       WHEN statusreceipt = 2 THEN costamountphysical + costamountadjustment
                       WHEN statusreceipt = 1 THEN costamountposted + costamountadjustment
                       WHEN statusissue = 2 THEN costamountphysical + costamountadjustment
                       WHEN statusissue = 1 THEN costamountposted + costamountadjustment
                       ELSE 0 END) / SUM(qty)
           ELSE 0 END                        AS AVG
FROM dbo.inventtrans
WHERE statusissue IN (0, 1, 2)
  AND statusreceipt IN (0, 1, 2)
  AND ISNULL(IsDelete, 0) = 0
GROUP BY itemid

-- @view dyn.ItemInventLocation
SELECT s.itemid                                AS [Articulo],
       et.name                                 AS [Descripción],
       e.searchname                            AS [Num.Parte],
       id.inventsiteid                         AS [Sucursal],
       id.inventlocationid                     AS [Almacen],
       il.name                                 AS [Name],
       ig.itemgroupid                          AS [Grupo],
       SUM(s.availphysical)                    AS [Disponible],
       MAX(s.postedvalue)                      AS [Costo Transaccion],
       MAX(ic.AVG)                             AS [Costo promedio],
       SUM(s.availphysical) * MAX(ic.AVG)      AS [Costo total],
       findim.Marca                            AS [Dimension],
       ec.name                                 AS [Cateogría],
       erv.TextValue                           AS [Marca],
       lpa.address                             AS [Address],
       s.dataareaid                            AS [Empresa]
FROM dbo.inventsum AS s
         INNER JOIN dbo.inventdim AS id
                    ON id.inventdimid = s.inventdimid AND ISNULL(id.IsDelete, 0) = 0
         INNER JOIN dbo.inventitemgroupitem ig
                    ON ig.itemid = s.itemid AND ISNULL(ig.IsDelete, 0) = 0
         INNER JOIN dbo.inventtable t
                    ON t.itemid = s.itemid AND ISNULL(t.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresproduct e
                   ON e.recid = t.product AND ISNULL(e.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresproducttranslation et
                   ON et.product = t.product AND UPPER(et.languageid) = 'ES-MX' AND ISNULL(et.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresproductcategory epc
                   ON epc.product = e.recid AND epc.categoryhierarchy = 5637144576 AND ISNULL(epc.IsDelete, 0) = 0
         LEFT JOIN dbo.ecorescategory ec
                   ON epc.category = ec.recid AND ISNULL(ec.IsDelete, 0) = 0
    -- Atributo marca del producto
         LEFT JOIN dyn.ProductBrand erv ON erv.RecId = e.recid
    -- Costo promedio del artículo
         LEFT JOIN dyn.ItemCost AS ic ON ic.ItemId = s.itemid
    -- Info del almacén
         LEFT JOIN dbo.inventlocation il
                   ON il.inventlocationid = id.inventlocationid
                       AND UPPER(il.dataareaid) = UPPER(s.dataareaid)
                       AND ISNULL(il.IsDelete, 0) = 0
         LEFT JOIN dbo.inventlocationlogisticslocation ll
                   ON ll.inventlocation = il.recid AND ll.isprimary = 1 AND ISNULL(ll.IsDelete, 0) = 0
         OUTER APPLY (SELECT TOP 1 lpa_inner.address
                      FROM dbo.logisticspostaladdress lpa_inner
                      WHERE lpa_inner.location = ll.location
                        AND GETDATE() BETWEEN lpa_inner.validfrom AND lpa_inner.validto
                        AND ISNULL(lpa_inner.IsDelete, 0) = 0
                      ORDER BY lpa_inner.validfrom DESC, lpa_inner.recid DESC) lpa
    -- Dimensiones financieras
         LEFT JOIN dyn.FinDim findim ON t.defaultdimension = findim.RECID
WHERE ISNULL(s.IsDelete, 0) = 0
GROUP BY s.itemid, et.name, e.searchname,
         id.inventsiteid, id.inventlocationid, il.name,
         ig.itemgroupid, findim.Marca, ec.name, s.dataareaid,
         erv.TextValue, lpa.address

-- @view dyn.ECOMProducts
SELECT s.dataareaid                 AS [DataAreaId],
       t.modifieddatetime           AS [MODIFIEDDATETIME],
       t.namealias                  AS [NameAlias],
       et.description               AS [Description],
       t.recid                      AS [RECID],
       s.itemid                     AS [Articulo],
       et.name                      AS [Descripción],
       e.searchname                 AS [Num.Parte],
       ig.itemgroupid               AS [Grupo],
       SUM(s.availphysical)         AS [Disponible],
       AVG(s.postedvalue)           AS [Costo],
       findim.Marca                 AS [Dimension],
       ec.name                      AS [Cateogría],
       erv.TextValue                AS [Marca]
FROM dbo.inventsum AS s
         INNER JOIN dbo.inventitemgroupitem ig
                    ON ig.itemid = s.itemid AND ISNULL(ig.IsDelete, 0) = 0
         INNER JOIN dbo.inventtable t
                    ON t.itemid = s.itemid AND ISNULL(t.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresproduct e
                   ON e.recid = t.product AND ISNULL(e.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresproducttranslation et
                   ON et.product = t.product AND UPPER(et.languageid) = 'ES-MX' AND ISNULL(et.IsDelete, 0) = 0
         LEFT JOIN dbo.ecoresproductcategory epc
                   ON epc.product = e.recid AND epc.categoryhierarchy = 5637144576 AND ISNULL(epc.IsDelete, 0) = 0
         LEFT JOIN dbo.ecorescategory ec
                   ON epc.category = ec.recid AND ISNULL(ec.IsDelete, 0) = 0
    -- Atributo marca del producto
         LEFT JOIN dyn.ProductBrand erv ON erv.RecId = e.recid
    -- Dimensiones financieras
         LEFT JOIN dyn.FinDim findim ON t.defaultdimension = findim.RECID
WHERE ISNULL(s.IsDelete, 0) = 0
GROUP BY s.dataareaid, t.modifieddatetime, t.namealias, et.description, t.recid,
         s.itemid, et.name, e.searchname, ig.itemgroupid, findim.Marca, ec.name,
         erv.TextValue
