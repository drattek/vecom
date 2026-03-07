package com.vegusa.middleware.repository.local;

import com.vegusa.middleware.entity.ProductAttributeValue;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.util.Date;
import java.util.List;

@Repository
public interface ProductAttributeValueRepository extends JpaRepository<ProductAttributeValue, Long> {
    @Query(value = "select * from productattributevalue where ProductAttributeId = ?1 and ItemId = ?2 and InterfaceId = ?3 and DataAreaId = ?4", nativeQuery = true)
    ProductAttributeValue getProductAttributeValue(String productAttributeId, String ItemId, String InterfaceId, String DataAreaId);

    @Query(value = "select * from productattributevalue where ProductAttributeId = ?1 and ItemId = ?2 and InterfaceId IN (?3) and DataAreaId = ?4 and Value IS NOT NULL order by FIELD(InterfaceId, ?5) limit 1", nativeQuery = true)
    ProductAttributeValue getProductAttributes(String productAttributeId, String itemId, List<String> interfaceIds, String dataAreaId, String priorityList);

    @Query(value = "select UpdatedAt from productattributevalue where ItemId = ?1 and DataAreaId = ?2 order by UpdatedAt desc limit 1", nativeQuery = true)
    Date getUpdatedDate(String itemId, String dataAreaId);

    @Modifying
    @Query(value = "delete from productattributevalue where ProductAttributeId = ?1 and ItemId = ?2 and InterfaceId = ?3 and DataAreaId = ?4", nativeQuery = true)
    void deleteProductAttributeValue(String productAttributeId, String ItemId, String InterfaceId, String DataAreaId);

    @Query(value = "select distinct ItemId from productattributevalue where DataAreaId = ?1 order by ItemId", nativeQuery = true)
    List<String> getProdAttValueItemIds(String DataAreaId);

    @Query(value = "select distinct ItemId from productattributevalue where DataAreaId = ?1 and ItemId in (" +
            "'MSB-0000106', 'MSB-0000211', 'MSB-0000212', 'MSB-0000266', 'MSB-0000276', 'MSB-0000285'," +
            "'MSB-0000394', 'MSB-0000399', 'MSB-0000408', 'MSB-0000411', 'MSB-0000412', 'MSB-0000485'," +
            "'MSB-0000529', 'MSB-0000545', 'MSB-0000546', 'MSB-0000547', 'MSB-0000634', 'MSB-0000745'," +
            "'MSB-0000749', 'MSB-0000782', 'MSB-0000794', 'MSB-0000797', 'MSB-0000799', 'MSB-0000802'," +
            "'MSB-0000826', 'MSB-0000835', 'MSB-0000837', 'MSB-0000883', 'MSB-0000900', 'MSB-0001023'," +
            "'MSB-0001028', 'MSB-0001030', 'MSB-0001036', 'MSB-0001037', 'MSB-0001041', 'MSB-0001051'," +
            "'MSB-0001052', 'MSB-0001256', 'MSB-0001257', 'MSB-0001265', 'MSB-0001295', 'MSB-0001326'," +
            "'MSB-0001393', 'MSB-0001398', 'MSB-0001399', 'MSB-0001468', 'MSB-0001688', 'MSB-0001697'," +
            "'MSB-0001744', 'MSB-0001790', 'MSB-0001791', 'MSB-0001815', 'MSB-0001826', 'MSB-0001843'," +
            "'MSB-0001845', 'MSB-0001853', 'MSB-0001857', 'MSB-0001862', 'MSB-0001864', 'MSB-0001879'," +
            "'MSB-0001880', 'MSB-0001881', 'MSB-0001882', 'MSB-0001898', 'MSB-0001924', 'MSB-0001926'," +
            "'MSB-0001927', 'MSB-0001937', 'MSB-0001949', 'MSB-0001962', 'MSB-0001981', 'MSB-0001982'," +
            "'MSB-0001983', 'MSB-0001990', 'MSB-0001991', 'MSB-0001992', 'MSB-0002005', 'MSB-0002009'," +
            "'MSB-0002010', 'MSB-0002011', 'MSB-0002012', 'MSB-0002020', 'MSB-0002036', 'MSB-0002037'," +
            "'MSB-0002038', 'MSB-0002051', 'MSB-0002055', 'MSB-0002061', 'MSB-0002076', 'MSB-0002085'," +
            "'MSB-0002126', 'MSB-0002139', 'MSB-0002172', 'MSB-0002173', 'MSB-0002176', 'MSB-0002185'," +
            "'MSB-0002186', 'MSB-0002191', 'MSB-0002193', 'MSB-0002214', 'MSB-0002223', 'MSB-0002227'," +
            "'MSB-0002228', 'MSB-0002240', 'MSB-0002255', 'MSB-0002263', 'MSB-0002303', 'MSB-0002307'," +
            "'MSB-0002373', 'MSB-0002374', 'MSB-0002378', 'MSB-0002380', 'MSB-0002389', 'MSB-0002430'," +
            "'MSB-0002431', 'MSB-0002445', 'MSB-0002446', 'MSB-0002452', 'MSB-0002472', 'MSB-0002473'," +
            "'MSB-0002479', 'MSB-0002482', 'MSB-0002531', 'MSB-0002536', 'MSB-0002546', 'MSB-0002549'," +
            "'MSB-0002592', 'MSB-0002593', 'MSB-0002597', 'MSB-0002602', 'MSB-0002603', 'MSB-0002606'," +
            "'MSB-0002620', 'MSB-0002625', 'MSB-0002665', 'MSB-0002700', 'MSB-0002729', 'MSB-0002740'," +
            "'MSB-0002744', 'MSB-0002754', 'MSB-0002771', 'MSB-0002773', 'MSB-0002777', 'MSB-0002785'," +
            "'MSB-0002804', 'MSB-0002806', 'MSB-0002815', 'MSB-0002817', 'MSB-0002818', 'MSB-0002820'," +
            "'MSB-0002841', 'MSB-0002852', 'MSB-0002905', 'MSB-0002906', 'MSB-0002911', 'MSB-0002917'," +
            "'MSB-0002921', 'MSB-0002929', 'MSB-0002933', 'MSB-0002944', 'MSB-0002945', 'MSB-0002970'," +
            "'MSB-0002973', 'MSB-0002976', 'MSB-0002988', 'MSB-0002990', 'MSB-0003000', 'MSB-0003001'," +
            "'MSB-0003029', 'MSB-0003120', 'MSB-0003169', 'MSB-0003216', 'MSB-0003460', 'MSB-0003462'," +
            "'MSB-0003484', 'MSB-0003501', 'MSB-0003515', 'MSB-0003521', 'MSB-0003550', 'MSB-0003764'," +
            "'MSB-0003765', 'MSB-0003782', 'MSB-0003838', 'MSB-0003996', 'MSB-0004002', 'MSB-0004018'," +
            "'MSB-0004022', 'MSB-0004679', 'MSB-0004731', 'MSB-0004930', 'MSB-0005087', 'MSB-0005102'," +
            "'MSB-0005351', 'MSB-0005456', 'MSB-0005459', 'MSB-0005553', 'MSB-0005650', 'MSB-0005845'," +
            "'MSB-0005848', 'MSB-0005904', 'MSB-0005976', 'MSB-0006063', 'MSB-0006097', 'MSB-0006126'," +
            "'MSB-0006170', 'MSB-0006257', 'MSB-0006443', 'MSB-0006448', 'MSB-0006541', 'MSB-0006542'," +
            "'MSB-0006581', 'MSB-0006794', 'MSB-0006903', 'MSB-0007107', 'MSB-0007157', 'MSB-0007711'," +
            "'MSB-0007809', 'MSB-0007992', 'MSB-0008400', 'MSB-0008420', 'MSB-0008502', 'MSB-0008503'," +
            "'MSB-0008511', 'MSB-0008570', 'MSB-0008721', 'MSB-0008722', 'MSB-0008749', 'MSB-0008819'," +
            "'MSB-0008971', 'MSB-0009544', 'MSB-0009549', 'MSB-0009582', 'MSB-0009725', 'MSB-0010186'," +
            "'MSB-0010553', 'MSB-0010695', 'MSB-0010791', 'MSB-0010873', 'MSB-0010895', 'MSB-0010993'," +
            "'MSB-0011117', 'MSB-0011191', 'MSB-0011280', 'MSB-0011289', 'MSB-0011295', 'MSB-0011436'," +
            "'MSB-0011456', 'MSB-0011620', 'MSB-0011679', 'MSB-0011828') order by ItemId", nativeQuery = true)
    List<String> getProdAttValueItemIds2(String DataAreaId);

}
