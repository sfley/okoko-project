<?php

 
$url_chaudiere = "http://X.X.X.X";
$port_json = "XXXX";
$password_json = "XXXXXXX";
 
 $param_list = ["log","log0","log1","log2","log3"]; 
 # $param_list = ["log"];
 $file = 'import_from_json.csv';

mb_internal_encoding("UTF-8");


function connectBase()
{
try
{
	// On se connecte à MySQL
	$bdd = new PDO('mysql:host=192.168.1.254;dbname=XXXX', 'XXXX', 'XXXXX');
	$bdd->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
	return $bdd;
}
catch(Exception $e)
{
	// En cas d'erreur, on affiche un message et on arrête tout
        die('Erreur : '.$e->getMessage());
}
}	
$bdd=connectBase();


 foreach ($param_list as $param) {
  
$url = $url_chaudiere.":".$port_json."/".$password_json."/".$param;
sleep(3);
// affichage d'un timestamp
 echo "<br>".date("d-m-Y H:i:s").": import de /".$param."<br>";
//Récupération des données json mise en forme du format degré et export en csv
$json = utf8_encode(file_get_contents($url));
//$json =str_replace("ý","°",$json);
$json =str_replace(",",".",$json); //formatage des nombres décimaux
file_put_contents($file, $json);


            
            // ouverture du fichier CSV en mode read-only
            $csvFile = fopen($file, 'r');
            
            // on saute la première ligne avec les en-têtes
            fgetcsv($csvFile,1000,";");
          
            // récupération des données dans le csv lignes par ligne 
            while(($line = fgetcsv($csvFile,1000,";")) !== FALSE)
			{
				
                // récupération des valeurs de colonne
for ($i = 0; $i < count($line); $i++) {
  $value = $line[$i];
  if ($i == 0) {
    $value = implode("-", array_reverse(explode(".", $value))); // reverse date
  }
  ${"value" . ($i + 1)} = $value;
}

  // on écrit la requête sql 


// Vérification si la date et l'heure existent déjà dans la table
        $check_sql = 'SELECT COUNT(*) FROM `json_import` WHERE `Date` = "'.$value1.'" AND `Heure` = "'.$value2.'"';
        $result_check = $bdd->query($check_sql)->fetchColumn();
		
        if ($result_check > 0) {// echo "La ligne ".$value1." ".$value2." existe déjà dans la table, insertion ignorée. <br/>";	
		                       }
						 
						 else {
							 
	if (empty($value1)) {
												echo "Erreur : La valeur de Date n'est pas valide dans : ".$value1." ".$value2."<br/>";
												} 
												else {						 
							 
            // on écrit la requête sql 
           
$sql = 'INSERT INTO `json_import`(`Date`, `Heure`, `AT_Temp_Ext`, `ATakt`, `KT_Ist_Temp_Chaudiere`, `KT_Soll_Temp_Chaudiere_Consigne`,
 `BR_Contact_Bruleur_on_off`, `Sperrzeit_Temps_de_blocage`, `PE1_BR1_ContactBruleur_on_off`, `HK1_VLIst_Temp_Depart`,
 `HK1_VLSoll_Temp_DepartConsigne`, `HK1_RTIst_Temp_Ambiante`, `HK1_RTSoll_Temp_AmbianteConsigne`,
 `HK1_Pumpe_Circulateur_Chauffage_on_off`, `HK1_Mischer`, `HK1_Fernb`, `HK1_Status`, `HK2_VLIst_Temp_Depart`,
 `HK2_VLSoll_Temp_Depart_Consigne`, `HK2_RTIst_Temp_Ambiante`, `HK2_RTSoll_Temp_Ambiante_Consigne`,
 `HK2_Pumpe_Circulateur_Chauffage_on_off`, `HK2_Mischer`, `HK2_Fernb`, `HK2_Status`, `WW1_EinTIst_Temp_ECS`,
 `WW1_AusTIst_Temp_ECS_arret`, `WW1_Soll_Temp_ECS_Consigne`, `WW1_Pompe_Circulateur_ECS`, `WW1_Status`, `Zubrp1Pumpe_Pompe_indefinie`,
 `PE1_KT_Temp_chaudiere`, `PE1_KT_SOLL_Temp_chaudiere_Consigne`, `PE1_UW_Freigabe`, `PE1_Modulation`, `PE1_FRTIst_Temp_Flamme`,
 `PE1_FRTSoll_Temp_Flamme_Consigne`, `PE1_FRTEnd`, `PE1_Einschublaufzeit_zs_Vis_Alimentation_marche`, `PE1_Pausenzeit_zs_Vis_Alimentation_pause`,
 `PE1_Luefterdrehzahl_Ventilation_Bruleur`, `PE1_Saugzugdrehzahl_Ventilation_fumee`, `PE1_UnterdruckIst_EH_Depression_Pa`,
 `PE1_UnterdruckSoll_EH_Depression_Consigne_Pa`, `PE1_Fuellstand_kg`, `PE1_Fuellstand_ZWB_kg`, `PE1_Status_Statut_chaudiere`,
 `PE1_Motor_ES_Moteur_alimentation_chaudiere_on_off`, `PE1_Motor_RA_Moteur_extraction_silo_on_off`, `PE1_Motor_RES1_Moteur_tremie_intermediaire`,
 `PE1_Motor_TURBINE_Moteur_Aspiration`, `PE1_Motor_ZUEND_Moteur_Allumage`, `PE1_Motor_UW_Pompe_du_circuit_primaire`, `PE1_Motor_AV`,
 `PE1_Motor_RES2`, `PE1_Motor_MA`, `PE1_Motor_RM_Moteur_ramonage`, `PE1_Motor_SM`, `PE1_CAPRA`, `PE1_CAPZB`, `PE1_AK`,
 `PE1_Saug_Int_min`, `PE1_DigIn1`, `PE1_DigIn2`, `Fehler1`, `Fehler2`, `Fehler3`)

 VALUES ("'.$value1.'","'.$value2.'","'.$value3.'","'.$value4.'","'.$value5.'","'.$value6.'","'.$value7.'","'.$value8.'","'.$value9.'","'.$value10.'",
 "'.$value11.'","'.$value12.'","'.$value13.'","'.$value14.'","'.$value15.'","'.$value16.'","'.$value17.'","'.$value18.'","'.$value19.'","'.$value20.'",
 "'.$value21.'","'.$value22.'","'.$value23.'","'.$value24.'","'.$value25.'","'.$value26.'","'.$value27.'","'.$value28.'","'.$value29.'","'.$value30.'",
 "'.$value31.'","'.$value32.'","'.$value33.'","'.$value34.'","'.$value35.'","'.$value36.'","'.$value37.'","'.$value38.'","'.$value39.'","'.$value40.'",
 "'.$value41.'","'.$value44.'","'.$value43.'","'.$value44.'","'.$value45.'","'.$value46.'","'.$value47.'","'.$value48.'","'.$value49.'","'.$value50.'",
 "'.$value51.'","'.$value55.'","'.$value53.'","'.$value54.'","'.$value55.'","'.$value56.'","'.$value57.'","'.$value58.'","'.$value59.'","'.$value60.'",
 "'.$value61.'","'.$value62.'","'.$value63.'","'.$value64.'","'.$value65.'","'.$value66.'","'.$value67.'")';


 // on insère les informations dans la table
 if(strpos($value1,'Datum') !== false){
    echo "Datum dans le log";
  } else{
	
try
{
	

 $req = $bdd->query($sql)or die('Erreur SQL !'.$sql.'<br>'.mysql_error());	
}
catch(Exception $e)
{
	// En cas d'erreur, on affiche un message et on arrête tout
        die('Erreur : '.$e->getMessage());
}


	
 
       
  }

 } 
						 }
 }
 }
 
 //$req->closeCursor();

  

 // mysql_close();  // on ferme la connexion 
 $bdd->connection = null;
           
  fclose($csvFile); // on ferme le fichier CSV 
            
       
       
    // affichage d'un timestamp
 echo "<br>".date("d-m-Y H:i:s").":fin des imports<br>";
 


?>